package monitorv3

import (
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type listObjectFunc func(string, metav1.ListOptions) (runtime.Object, error)
type watchObjectFunc func(string, metav1.ListOptions) (watch.Interface, error)

type SecretEventHandlerRegistration interface {
	cache.ResourceEventHandlerRegistration

	GetKey() ObjectKey
	GetHandler() cache.ResourceEventHandlerRegistration
}

type SecretMonitor interface {
	AddEventHandler(namespace, name string, handler cache.ResourceEventHandler) (SecretEventHandlerRegistration, error)

	RemoveEventHandler(SecretEventHandlerRegistration) error

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
	// listObject  listObjectFunc
	// watchObject watchObjectFunc

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

// create/update secret watch
// TODO: Think what is the best place to create ObjectKey{},
func (s *sm) AddEventHandler(namespace, name string, handler cache.ResourceEventHandler) (SecretEventHandlerRegistration, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// name is a combination of "routename_secretname"
	// Inside a namespace multiple route can refer a common secret.
	// Hence route1_secret and route2_secret should denote separate key,
	// so separate monitors inside a namespace.
	key := ObjectKey{Namespace: namespace, Name: name}
	klog.Info("ObjectKey", key)

	// Check if monitor / watch already exists
	m, exists := s.monitors[key]

	// TODO refactor this later
	if !exists {
		// fieldSelector := fields.Set{"metadata.name": secretName}.AsSelector().String()
		// listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
		// 	options.FieldSelector = fieldSelector
		//
		// 	klog.Info(fieldSelector)
		// 	return s.listObject(namespace, options)
		// }
		// watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		// 	options.FieldSelector = fieldSelector
		//
		// 	klog.Info(fieldSelector)
		// 	return s.watchObject(namespace, options)
		// }
		//
		// store, informer := cache.NewInformer(
		// 	&cache.ListWatch{ListFunc: listFunc, WatchFunc: watchFunc},
		// 	&v1.Secret{},
		// 	0, handler)

		// extract secretName and create a monitor for only that secret
		// TODO: put inside if statement, to handle index out of bound
		// TODO: can use GetItemKey()
		secretName := strings.Split(name, "_")[1]

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
	klog.Info("numHandlers", m.numHandlers.Load())
	if m.numHandlers.Load() <= 0 {
		if !m.Stop() {
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
	klog.Info("Key for getsecret ", key)

	// check if secret watch exists
	m, exists := s.monitors[key]

	if !exists {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, m.GetItemKey())
	}

	// should take key as argument
	uncast, exists, err := m.GetItem()
	if !exists {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, m.GetItemKey())
	}

	if err != nil {
		return nil, err
	}

	ret, ok := uncast.(*v1.Secret)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", uncast)
	}

	return ret, nil

}
