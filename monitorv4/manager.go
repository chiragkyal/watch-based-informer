package secret

import (
	"context"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type Manager struct {
	monitor SecretMonitor
	// map of handlerRegistrations
	registeredHandlers map[string]SecretEventHandlerRegistration

	lock sync.RWMutex

	// monitors are the producer of the resourceChanges queue
	resourceChanges workqueue.RateLimitingInterface

	// secretHandler cache.ResourceEventHandlerFuncs
	secretHandler cache.ResourceEventHandler
}

func NewManager(kubeClient *kubernetes.Clientset, queue workqueue.RateLimitingInterface) *Manager {
	return &Manager{
		monitor:            NewSecretMonitor(kubeClient),
		lock:               sync.RWMutex{},
		resourceChanges:    queue,
		registeredHandlers: make(map[string]SecretEventHandlerRegistration),

		// default secret handler
		secretHandler: nil,
	}
}

func (m *Manager) WithSecretHandler(handler cache.ResourceEventHandlerFuncs) *Manager {
	m.secretHandler = handler
	return m
}

func (m *Manager) Queue() workqueue.RateLimitingInterface {
	return m.resourceChanges
}

func (m *Manager) RegisterRoute(ctx context.Context, namespace, routeName, secretName string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// each route (namespace/routeName) should be registered only once with any secret.
	// Note: inside a namespace multiple different routes can be registered(watch) with a common secret
	key := generateKey(namespace, routeName)
	if _, exists := m.registeredHandlers[key]; exists {
		return apierrors.NewInternalError(fmt.Errorf("route already registered with key %s", key))
	}

	handlerRegistration, err := m.monitor.AddSecretEventHandler(ctx, namespace, secretName, m.secretHandler)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	m.registeredHandlers[key] = handlerRegistration
	klog.Info(fmt.Sprintf("secret manager registered route for key %s with secret %s", key, secretName))

	return nil
}

func (m *Manager) UnregisterRoute(namespace, routeName string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	key := generateKey(namespace, routeName)

	// grab registered handler
	handlerRegistration, exists := m.registeredHandlers[key]
	if !exists {
		return apierrors.NewInternalError(fmt.Errorf("no handler registered with key %s", key))
	}

	klog.Info("trying to remove handler with key", key)
	err := m.monitor.RemoveSecretEventHandler(handlerRegistration)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	// delete registered handler from the map
	delete(m.registeredHandlers, key)
	klog.Info("secret manager unregistered route ", key)

	return nil
}

func (m *Manager) GetSecret(namespace, routeName string) (*v1.Secret, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	key := generateKey(namespace, routeName)

	handlerRegistration, exists := m.registeredHandlers[key]
	if !exists {
		return nil, apierrors.NewInternalError(fmt.Errorf("no handler registered with key %s", key))
	}

	obj, err := m.monitor.GetSecret(handlerRegistration)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func generateKey(namespace, route string) string {
	return fmt.Sprintf("%s/%s", namespace, route)
}
