package monitorv3

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

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

func Testxxx(t *testing.T) {
	scenarios := []struct {
		name string
	}{
		{},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
		})
	}
}

var (
	testNamespacee  = "testNamespace"
	testSecretNamee = "testSecretName"
	testRouteNamee  = "testRouteName"
)

func fakeMonitor_(ctx context.Context, fakeKubeClient *fake.Clientset, key ObjectKey) *singleItemMonitor {
	sharedInformer := fakeInformer_(ctx, fakeKubeClient, key.Namespace)
	return newSingleItemMonitor(key, sharedInformer)
}

func fakeInformer_(ctx context.Context, fakeKubeClient *fake.Clientset, namespace string) cache.SharedInformer {
	return cache.NewSharedInformer(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return fakeKubeClient.CoreV1().Secrets(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return fakeKubeClient.CoreV1().Secrets(namespace).Watch(ctx, options)
		},
	},
		&corev1.Secret{},
		1*time.Second,
	)
}

func fakeSecret_(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"test": {},
		},
	}
}

func TestMonitorr(t *testing.T) {
	fakeKubeClient := fake.NewSimpleClientset(fakeSecret_(testNamespacee, testSecretNamee))
	queue := make(chan string)
	ctx, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	sharedInformer := fakeInformer_(ctx, fakeKubeClient, testNamespacee)
	routeSecetName := fmt.Sprintf("%s_%s", testRouteNamee, testSecretNamee)
	singleItemMonitor := newSingleItemMonitor(NewObjectKey(testNamespacee, routeSecetName), sharedInformer)

	go singleItemMonitor.StartInformer()
	if !cache.WaitForCacheSync(ctx.Done(), singleItemMonitor.HasSynced) {
		t.Fatal("cache not synced yet")
	}

	handlerfunc := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				t.Errorf("invalid object")
			}
			queue <- secret.Name
		},
	}
	var intr cache.ResourceEventHandler
	intr = handlerfunc
	handlerRegistration, err := singleItemMonitor.AddEventHandler(intr)
	if err != nil {
		t.Errorf("got error %v", err)
	}

	if singleItemMonitor.numHandlers.Load() != 1 {
		t.Errorf("expected %d handler got %d", 1, singleItemMonitor.numHandlers.Load())
	}

	select {
	case s := <-queue:
		if s != testSecretNamee {
			t.Errorf("expected %s got %s", testSecretNamee, s)
		}

		// get secret
		uncast, exists, err := singleItemMonitor.GetItem(testSecretNamee)
		if err != nil {
			t.Error(err)
		}
		if !exists {
			t.Error("secret does not exist")
		}
		ret, ok := uncast.(*v1.Secret)
		if !ok {
			t.Errorf("unexpected type: %T", uncast)
		}
		if s != ret.Name {
			t.Errorf("expected %s got %s", ret.Name, s)
		}

		err = singleItemMonitor.RemoveEventHandler(handlerRegistration)
		if err != nil {
			t.Errorf("got error : %v", err.Error())
		}
		if singleItemMonitor.numHandlers.Load() != 0 {
			t.Errorf("expected %d handler got %d", 0, singleItemMonitor.numHandlers.Load())
		}
		if !singleItemMonitor.StopInformer() {
			t.Error("failed to stop informer")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("test timeout")
	}
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
