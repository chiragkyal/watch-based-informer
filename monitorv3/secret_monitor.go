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
	AddEventHandler(namespace, routeSecretName string, handler cache.ResourceEventHandler) (SecretEventHandlerRegistration, error)
	//
	RemoveEventHandler(SecretEventHandlerRegistration) error
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

type sm struct {
	kubeClient kubernetes.Interface

	lock sync.RWMutex
	// monitors is map of singleItemMonitor. Each singleItemMonitor monitors/watches
	// a secret through individual informer.
	monitors map[ObjectKey]*singleItemMonitor
}

func NewSecretMonitor(kubeClient *kubernetes.Clientset) SecretMonitor {
	return &sm{
		kubeClient: kubeClient,
		monitors:   map[ObjectKey]*singleItemMonitor{},
	}
}

// create/update secret watch.
// routeSecretName is a combination of "routename_secretname"
func (s *sm) AddEventHandler(namespace, routeSecretName string, handler cache.ResourceEventHandler) (SecretEventHandlerRegistration, error) {
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

		// create a single secret monitor
		sharedInformer := cache.NewSharedInformer(
			cache.NewListWatchFromClient(
				s.kubeClient.CoreV1().RESTClient(),
				"secrets",
				namespace,
				fields.OneTermEqualSelector("metadata.name", secretName),
			),
			&corev1.Secret{},
			0,
		)

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
func (s *sm) RemoveEventHandler(handlerRegistration SecretEventHandlerRegistration) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := handlerRegistration.GetKey()
	// check if watch already exists for key
	m, exists := s.monitors[key]
	if !exists {
		// already removed
		klog.Info("handler already removed, key", key)
		return nil
	}

	klog.Info("removing secret handler, key", key)
	if err := m.RemoveEventHandler(handlerRegistration); err != nil {
		klog.Error(err)
		return err
	}
	klog.Info("secret handler removed", " item key", key)

	// stop the watch/informer if there is no handler
	if m.numHandlers.Load() <= 0 {
		if !m.StopInformer() {
			klog.Error("failed to stop secret informer")
		}
		delete(s.monitors, key)
		klog.Info("secret informer stopped ", " item key ", key)
	}

	return nil
}

// Get the secret object from informer's cache
func (s *sm) GetSecret(handlerRegistration SecretEventHandlerRegistration) (*v1.Secret, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

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
