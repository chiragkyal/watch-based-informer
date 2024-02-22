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
	HandleRoute( /*eventType watch.EventType,*/ route *v1.Pod /*route *routev1.Route*/) error
	GetResourceKey() string
	// GetEventType() watch.EventType
}

type RouteEvent struct {
	eventType     watch.EventType
	resourceKey   string
	secretManager *secret.Manager
}

func NewRouteEvent(eventType watch.EventType, resourceKey string, secretManager *secret.Manager) Handler {
	return &RouteEvent{
		eventType:     eventType,
		resourceKey:   resourceKey,
		secretManager: secretManager,
	}
}

func (re *RouteEvent) GetResourceKey() string {
	return re.resourceKey
}

// func (re *RouteEvent) GetEventType() watch.EventType {
// 	return re.eventType
// }

// TODO: eventType can also be directly accessed?
func (re *RouteEvent) HandleRoute( /*eventType watch.EventType,*/ route *v1.Pod /*route *routev1.Route*/) error {

	switch re.eventType {

	case watch.Added:
		err := localRegisterRoute(re.secretManager, route)
		if err != nil {
			klog.Error("failed to register route")
			return err
		}

		// Extra : Print secret Content
		s, err := re.secretManager.GetSecret(route.Namespace, route.Name)
		if err != nil {
			klog.Error(err)
			return err
		}
		klog.Info("fetching secret data ", s.Data)

	// case watch.Modified:
	// 	if len(tls.Certificate) > 0 && tls.ExternalCertificate != nil && len(tls.ExternalCertificate.Name) == 0 {
	// 		sm.secretManager.UnregisterRoute(route)
	// 	} else if len(route.Spec.TLS.Certificate) == 0 && tls.ExternalCertificate != nil && len(tls.ExternalCertificate.Name) > 0 {
	// 		sm.secretManager.RegisterRoute(route)
	// 	}

	case watch.Deleted:
		err := re.secretManager.UnregisterRoute(route.Namespace, route.Name)
		if err != nil {
			klog.Error(err)
			return err
		}
	default:
		// we are not supporting watch.Modified
		klog.Error("invalid eventType", re.eventType)
	}
	return nil
}

func localRegisterRoute(secretManager *secret.Manager, route *v1.Pod /*routev1.Route*/) error {
	secreth := generateSecretHandler(secretManager, route)
	secretManager.WithSecretHandler(secreth)
	return secretManager.RegisterRoute(context.Background(), route.Namespace, route.Name, getReferenceSecret(route))

}

func generateSecretHandler(secretManager *secret.Manager, route *v1.Pod /*routev1.Route*/) cache.ResourceEventHandlerFuncs {
	// secret handler
	secreth := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(obj)
			secret := obj.(*v1.Secret)
			klog.Info("Secret added ", "obj ", secret.ResourceVersion, " key ", key, " For ", route.Namespace+"/"+route.Name)
			// secretManager.Queue().Add("From secreth " + route.Namespace + "/" + route.Name)
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(new)
			secretOld := old.(*v1.Secret)
			secretNew := new.(*v1.Secret)
			klog.Info("Secret updated ", "old ", secretOld.ResourceVersion, " new ", secretNew.ResourceVersion, " key ", key, " For ", route.Namespace+"/"+route.Name)
			// secretManager.Queue().Add("From secreth " + route.Namespace + "/" + route.Name)
		},
		DeleteFunc: func(obj interface{}) {
			key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			secret := obj.(*v1.Secret)
			klog.Info("Secret deleted ", " obj ", secret.ResourceVersion, " key ", key, " For ", route.Namespace+"/"+route.Name)
			// secretManager.Queue().Add("From secreth " + route.Namespace + "/" + route.Name)
		},
	}

	return secreth
}
