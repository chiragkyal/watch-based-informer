// package secret
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

var (
	testNamespace  = "testNamespace"
	testSecretName = "testSecretName"
	testRouteName  = "testRouteName"
)

func newInformer(ctx context.Context, fakeKubeClient *fake.Clientset, namespace string) cache.SharedInformer {
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

func fakeSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"test": {},
		},
	}
}

/*
- Scenarios
	- Informer is running properly
	- Informer is stopped properly
	 	- Dual stop should return false
	- AddEventHandler should increase the count by 1, check error
	- RemoveEventHandler should decrease the count by 1, check error
*/

func TestMonitor(t *testing.T) {
	fakeKubeClient := fake.NewSimpleClientset(fakeSecret(testNamespace, testSecretName))
	queue := make(chan string)
	ctx, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	sharedInformer := newInformer(ctx, fakeKubeClient, testNamespace)
	name := fmt.Sprintf("%s_%s", testRouteName, testSecretName)
	singleItemMonitor := newSingleItemMonitor(ObjectKey{Name: name, Namespace: testNamespace}, sharedInformer)

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
		UpdateFunc: func(oldObj, newObj interface{}) {},
		DeleteFunc: func(obj interface{}) {},
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
		if s != testSecretName {
			t.Errorf("expected %s got %s", testSecretName, s)
		}

		// get secret
		uncast, exists, err := singleItemMonitor.GetItem(testSecretName)
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
		if !singleItemMonitor.Stop() {
			t.Error("failed to stop informer")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("test timeout")
	}
}
