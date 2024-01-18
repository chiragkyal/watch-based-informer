package monitorv3

import (
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type SecretEventHandlerRegistration interface {
	cache.ResourceEventHandlerRegistration

	GetKey() ObjectKey
	GetHandler() cache.ResourceEventHandlerRegistration
}

type SecretMonitor interface {
	//
	AddSecretEventHandler(namespace, routeSecretName string, handler cache.ResourceEventHandler) (SecretEventHandlerRegistration, error)
	//
	RemoveSecretEventHandler(SecretEventHandlerRegistration) error
	//
	GetSecret(SecretEventHandlerRegistration) (*v1.Secret, error)
}

type secretEventHandlerRegistration struct {
	cache.ResourceEventHandlerRegistration
	// objectKey will populate during AddEventHandler, and will be used during RemoveEventHandler
	objectKey ObjectKey
}

func (r *secretEventHandlerRegistration) GetKey() ObjectKey {
	return r.objectKey
}

func (r *secretEventHandlerRegistration) GetHandler() cache.ResourceEventHandlerRegistration {
	return r.ResourceEventHandlerRegistration
}

type secretMonitor struct {
	kubeClient kubernetes.Interface

	lock sync.RWMutex
	// monitors is map of singleItemMonitor. Each singleItemMonitor monitors/watches
	// a secret through individual informer.
	monitors map[ObjectKey]*singleItemMonitor
}

func NewSecretMonitor(kubeClient kubernetes.Interface) SecretMonitor {
	return &secretMonitor{
		kubeClient: kubeClient,
		monitors:   map[ObjectKey]*singleItemMonitor{},
	}
}

// create/update secret watch.
// routeSecretName is a combination of "routename_secretname"
func (s *secretMonitor) AddSecretEventHandler(namespace, routeSecretName string, handler cache.ResourceEventHandler) (SecretEventHandlerRegistration, error) {
	return s.addSecretEventHandler(namespace, routeSecretName, handler, nil)
}

// addSecretEventHandler should only be used directly for tests. For production use AddSecretEventHandler().
// createInformerFn helps in mocking sharedInformer for unit tests.
func (s *secretMonitor) addSecretEventHandler(namespace, routeSecretName string, handler cache.ResourceEventHandler, createInformerFn func() cache.SharedInformer) (SecretEventHandlerRegistration, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !validateName(routeSecretName) {
		return nil, fmt.Errorf("invalid routeSecretName combination : %s", routeSecretName)
	}

	// Inside a namespace multiple route can refer a common secret.
	// Hence route1_secret and route2_secret should denote separate key,
	// so separate monitors inside a namespace.
	key := NewObjectKey(namespace, routeSecretName)

	// Check if monitor / watch already exists
	m, exists := s.monitors[key]
	if !exists {
		secretName, err := getSecretName(routeSecretName, true)
		if err != nil {
			return nil, err
		}

		var sharedInformer cache.SharedInformer
		if createInformerFn == nil {
			// create a single secret monitor
			sharedInformer = cache.NewSharedInformer(
				cache.NewListWatchFromClient(
					s.kubeClient.CoreV1().RESTClient(),
					"secrets",
					namespace,
					fields.OneTermEqualSelector("metadata.name", secretName),
				),
				&corev1.Secret{},
				0,
			)
		} else {
			// only for testability
			klog.Warning("creating informer for testability")
			sharedInformer = createInformerFn()
		}

		m = newSingleItemMonitor(key, sharedInformer)
		go m.StartInformer()

		// add item key to monitors map // add watch to the list
		s.monitors[key] = m

		klog.Info("secret informer started", " item key ", key)
	}

	klog.Info("secret handler added", " item key ", key)

	return m.AddEventHandler(handler) // also populate key inside handlerRegistartion
}

// Remove secret watch
func (s *secretMonitor) RemoveSecretEventHandler(handlerRegistration SecretEventHandlerRegistration) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if handlerRegistration == nil {
		return fmt.Errorf("secret handler is nil")
	}
	key := handlerRegistration.GetKey()

	// check if watch already exists for key
	m, exists := s.monitors[key]
	if !exists {
		klog.Info("secret monitor already removed", " item key", key)
		return nil
	}

	if err := m.RemoveEventHandler(handlerRegistration); err != nil {
		return err
	}
	klog.Info("secret handler removed", " item key", key)

	// stop informer if there is no handler
	if m.numHandlers.Load() <= 0 {
		if !m.StopInformer() {
			klog.Warning("secret informer already stopped", " item key", key)
		}
		delete(s.monitors, key)
		klog.Info("secret informer stopped", " item key ", key)
	}

	return nil
}

// Get the secret object from informer's cache
func (s *secretMonitor) GetSecret(handlerRegistration SecretEventHandlerRegistration) (*v1.Secret, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if handlerRegistration == nil {
		return nil, fmt.Errorf("secret handler is nil")
	}
	key := handlerRegistration.GetKey()

	// check if secret watch exists
	m, exists := s.monitors[key]
	if !exists {
		return nil, fmt.Errorf("secret monitor doesn't exist for key %v", key)
	}

	secretName, err := getSecretName(key.Name, false)
	if err != nil {
		return nil, err
	}

	uncast, exists, err := m.GetItem(secretName)
	if !exists {
		return nil, fmt.Errorf("secret %s doesn't exist in cache", secretName)
	}

	if err != nil {
		return nil, err
	}

	secret, ok := uncast.(*v1.Secret)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", uncast)
	}

	return secret, nil
}

func getSecretName(routeSecretName string, isValidated bool) (string, error) {
	if !isValidated {
		if !validateName(routeSecretName) {
			return "", fmt.Errorf("invalid routeSecretName combination : %s", routeSecretName)
		}
	}
	return strings.Split(routeSecretName, "_")[1], nil
}

// validateName checks whether 'routeSecretName' follows
// the combination of routeName_secretName
func validateName(routeSecretName string) bool {
	if keys := strings.Split(routeSecretName, "_"); len(keys) == 2 {
		if len(keys[0]) > 0 && len(keys[1]) > 0 {
			return true
		}
		return false
	}
	return false
}
