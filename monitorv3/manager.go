package monitorv3

import (
	"fmt"
	"sync"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type Manager struct {
	monitor SecretMonitor
	// map of handlerRegistrations
	registeredHandlers map[ObjectKey]SecretEventHandlerRegistration

	lock sync.RWMutex

	// monitors are the producer of the resourceChanges queue
	resourceChanges workqueue.RateLimitingInterface

	secretHandler cache.ResourceEventHandlerFuncs
}

func NewManager(kubeClient *kubernetes.Clientset, queue workqueue.RateLimitingInterface) *Manager {
	return &Manager{
		monitor:            NewSecretMonitor(kubeClient),
		lock:               sync.RWMutex{},
		resourceChanges:    queue,
		registeredHandlers: make(map[ObjectKey]SecretEventHandlerRegistration),

		// default secret handler
		secretHandler: cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) {},
			UpdateFunc: func(oldObj, newObj interface{}) {},
			DeleteFunc: func(obj interface{}) {},
		},
	}
}

func (m *Manager) WithSecretHandler(handler cache.ResourceEventHandlerFuncs) *Manager {
	m.secretHandler = handler
	return m
}

func (m *Manager) RegisterRoute(parent *routev1.Route) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	secretName := parent.Spec.TLS.ExternalCertificate.Name
	key := generateKey(parent.Namespace, parent.Name, secretName)

	handlerRegistration, err := m.monitor.AddSecretEventHandler(key.Namespace, key.Name, m.secretHandler)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	m.registeredHandlers[key] = handlerRegistration
	klog.Info("secret manager registered route ", key)

	return nil
}

func (m *Manager) UnregisterRoute(parent *routev1.Route) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	secretName := parent.Spec.TLS.ExternalCertificate.Name
	key := generateKey(parent.Namespace, parent.Name, secretName)
	klog.Info("trying to UnregisterRoute with key", key)

	// grab registered handler
	handlerRegistration, exists := m.registeredHandlers[key]
	if !exists {
		//return apierrors.NewNotFound(schema.GroupResource{Resource: "routes"}, key)
		return apierrors.NewInternalError(fmt.Errorf("no handler registered with key %s", key.Name))
	}

	klog.Info("trying to remove handler with key", key)
	err := m.monitor.RemoveSecretEventHandler(handlerRegistration)
	if err != nil {
		// return apierrors.NewNotFound(schema.GroupResource{Resource: "routes"}, key)
		return apierrors.NewInternalError(err)
	}

	// delete registered handler from the map
	delete(m.registeredHandlers, key)
	klog.Info("secret manager unregistered route ", key)

	return nil
}

// Get secret object from informer's cache.
// It will first check HasSynced()
func (m *Manager) GetSecret(parent *routev1.Route) (*v1.Secret, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	secretName := parent.Spec.TLS.ExternalCertificate.Name

	key := generateKey(parent.Namespace, parent.Name, secretName)
	handlerRegistration, exists := m.registeredHandlers[key]

	if !exists {
		//return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, key.Name)
		return nil, apierrors.NewInternalError(fmt.Errorf("no handler registered with key %s", key.Name))
	}

	// check if informer store synced or not, to load secret
	if err := wait.PollImmediate(10*time.Millisecond, time.Second, func() (done bool, err error) { return handlerRegistration.HasSynced(), nil }); err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	obj, err := m.monitor.GetSecret(handlerRegistration)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func generateKey(namespace, routeName, secretName string) ObjectKey {
	return NewObjectKey(
		namespace,
		fmt.Sprintf("%s_%s", routeName, secretName),
	)
}
