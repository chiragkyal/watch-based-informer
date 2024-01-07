package monitorv3

import "fmt"

type RouteSecret interface {
	ToString() string
}

type routeSecret struct {
	routeName  string
	secretName string
}

func NewRouteSecret(routeName, secretName string) RouteSecret {
	return &routeSecret{
		routeName:  routeName,
		secretName: secretName,
	}
}

func (rs *routeSecret) ToString() string {
	return fmt.Sprintf("%s_%s", rs.routeName, rs.secretName)
}

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

// func (c *singleItemMonitor) GetByKey(name string) (interface{}, bool, error) {
// 	return c.store.GetByKey(name)
// }
//
// func (c *singleItemMonitor) GetKey() objectKey {
// 	return c.key
// }

// key returns key of an object with a given name and namespace.
// This has to be in-sync with cache.MetaNamespaceKeyFunc.
// func (c *singleItemMonitor) Key(namespace, name string) string {
// 	if len(namespace) > 0 {
// 		return namespace + "/" + name
// 	}
// 	return name
// }
