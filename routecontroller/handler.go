package main

import (
	"context"

	secret "github.com/chiragkyal/watch-based-informer/monitorv4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type Handler interface {
	HandleRoute( /*eventType watch.EventType,route *v1.Pod*/ ) error
	// GetResourceKey() string
	// GetEventType() watch.EventType
}

type RouteEvent struct {
	eventType watch.EventType
	// resourceKey   string
	route         *v1.ConfigMap
	secretManager *secret.Manager
}

func NewRouteEvent(eventType watch.EventType, route *v1.ConfigMap, secretManager *secret.Manager) Handler {
	return &RouteEvent{
		eventType:     eventType,
		route:         route,
		secretManager: secretManager,
	}
}

// func (re *RouteEvent) GetResourceKey() string {
// 	return re.resourceKey
// }

// func (re *RouteEvent) GetEventType() watch.EventType {
// 	return re.eventType
// }

func (re *RouteEvent) HandleRoute() error {

	route := re.route
	eventType := re.eventType
	switch eventType {

	case watch.Added:
		err := localRegisterRoute(re.secretManager, route)
		if err != nil {
			klog.Error("failed to register route")
			return err
		}

	case watch.Modified:
		// Update the route content to serve the certificate
		s, err := re.secretManager.GetSecret(route.Namespace, route.Name)
		// TODO: what will happen if secret got deleted? should the route still serve the old certificate?
		// Now err will return, secret not found
		if err != nil {
			klog.Error(err)
			return err
		}
		klog.Info("fetching secret data ", s.Data)

	case watch.Deleted:
		err := re.secretManager.UnregisterRoute(route.Namespace, route.Name)
		if err != nil {
			klog.Error(err)
			return err
		}
	default:
		klog.Error("invalid eventType", eventType)
	}
	return nil
}

func localRegisterRoute(secretManager *secret.Manager, route *v1.ConfigMap /*routev1.Route*/) error {
	secreth := generateSecretHandler(secretManager, route)
	secretManager.WithSecretHandler(secreth)
	return secretManager.RegisterRoute(context.Background(), route.Namespace, route.Name, getReferenceSecret(route))

}

func generateSecretHandler(secretManager *secret.Manager, route *v1.ConfigMap /*routev1.Route*/) cache.ResourceEventHandlerFuncs {
	// secret handler
	secreth := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// key, _ := cache.MetaNamespaceKeyFunc(obj)
			secret := obj.(*v1.Secret)
			klog.Info("Secret added ", "obj ", secret.ResourceVersion, " key ", secret.Name, " For ", route.Namespace+"/"+route.Name)
			secretManager.Queue().Add(NewRouteEvent(watch.Modified, route, secretManager))
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			// key, _ := cache.MetaNamespaceKeyFunc(new)
			secretOld := old.(*v1.Secret)
			secretNew := new.(*v1.Secret)
			klog.Info("Secret updated ", "old ", secretOld.ResourceVersion, " new ", secretNew.ResourceVersion, " key ", secretNew.Name, " For ", route.Namespace+"/"+route.Name)
			secretManager.Queue().Add(NewRouteEvent(watch.Modified, route, secretManager))
		},
		DeleteFunc: func(obj interface{}) {
			// key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			secret := obj.(*v1.Secret)
			klog.Info("Secret deleted ", " obj ", secret.ResourceVersion, " key ", secret.Name, " For ", route.Namespace+"/"+route.Name)
			secretManager.Queue().Add(NewRouteEvent(watch.Modified, route, secretManager))
		},
	}

	return secreth
}
