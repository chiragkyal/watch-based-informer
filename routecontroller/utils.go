package main

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func emptyResource() *v1.ConfigMap {
	return &v1.ConfigMap{}
}
func getResource(obj interface{}) *v1.ConfigMap {
	return obj.(*v1.ConfigMap)
}

func getListWatcher(clientset *kubernetes.Clientset, namespace string) *cache.ListWatch {
	// routeListWatcher := cache.NewListWatchFromClient(routev1client.NewForConfigOrDie(config).RouteV1().RESTClient(), "routes", namespace, fields.Everything())
	return cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "configmaps", namespace, fields.Everything())
}

func getReferenceSecret(route *v1.ConfigMap /*routev1.Route*/) string {
	// route.Spec.TLS.ExternalCertificate.Name
	// secretName := route.Spec.Containers[0].Env[0].ValueFrom.SecretKeyRef.Name
	secretName := route.Data["secretName"]
	klog.Info("Referenced secretName: ", secretName)
	return secretName
}
