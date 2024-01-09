// package secret
package monitorv3

import (
	"context"
	"fmt"
	"reflect"
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

// Remove global variable, can cause issue
var (
	testNamespace  = "testNamespace"
	testSecretName = "testSecretName"
	testRouteName  = "testRouteName"
)

func fakeMonitor(ctx context.Context, fakeKubeClient *fake.Clientset, key ObjectKey) *singleItemMonitor {
	sharedInformer := fakeInformer(ctx, fakeKubeClient, key.Namespace)
	return newSingleItemMonitor(key, sharedInformer)
}

func fakeInformer(ctx context.Context, fakeKubeClient *fake.Clientset, namespace string) cache.SharedInformer {
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
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"test": {},
		},
	}
}

/*
- Scenarios

	- GetItem when item is not present, unable to cast, error
*/

func TestMonitor(t *testing.T) {
	fakeKubeClient := fake.NewSimpleClientset(fakeSecret(testNamespace, testSecretName))
	queue := make(chan string)
	ctx, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	sharedInformer := fakeInformer(ctx, fakeKubeClient, testNamespace)
	routeSecetName := fmt.Sprintf("%s_%s", testRouteName, testSecretName)
	singleItemMonitor := newSingleItemMonitor(NewObjectKey(testNamespace, routeSecetName), sharedInformer)

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
		if !singleItemMonitor.StopInformer() {
			t.Error("failed to stop informer")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("test timeout")
	}
}

func TestStartInformer(t *testing.T) {
	scenarios := []struct {
		name      string
		isClosed  bool
		expectErr bool
	}{
		{
			name:      "pass closed channel into informer",
			isClosed:  true,
			expectErr: true,
		},
		{
			name:      "pass unclosed channel into informer",
			isClosed:  false,
			expectErr: false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset()
			monitor := fakeMonitor(context.TODO(), fakeKubeClient, ObjectKey{})
			if s.isClosed {
				close(monitor.stopCh)
			}
			go monitor.StartInformer()

			select {
			// this case will execute if stopCh is closed
			case <-monitor.stopCh:
				if !s.expectErr {
					t.Error("informer is not running")
				}
			default:
				t.Log("informer is running")
			}
		})
	}
}

func TestStopInformer(t *testing.T) {
	scenarios := []struct {
		name           string
		alreadyStopped bool
		expect         bool
	}{
		{
			name:           "stopping already stopped informer",
			alreadyStopped: true,
			expect:         false,
		},
		{
			name:           "correctly stopped informer",
			alreadyStopped: false,
			expect:         true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset()
			monitor := fakeMonitor(context.TODO(), fakeKubeClient, ObjectKey{})
			go monitor.StartInformer()

			if s.alreadyStopped {
				monitor.StopInformer()
			}
			if monitor.StopInformer() != s.expect {
				t.Error("unexpected result")
			}

			select {
			// this case will execute if stopCh is closed
			case <-monitor.stopCh:
				t.Log("informer successfully stopped")
			default:
				t.Error("informer is still running")
			}
		})
	}
}

func TestAddEventHandler(t *testing.T) {
	fakeKubeClient := fake.NewSimpleClientset()
	key := NewObjectKey("namespace", "name")
	monitor := fakeMonitor(context.TODO(), fakeKubeClient, key)

	handlerRegistration, err := monitor.AddEventHandler(cache.ResourceEventHandlerFuncs{})
	if err != nil {
		t.Errorf("got error %v", err)
	}
	if monitor.numHandlers.Load() != 1 {
		t.Errorf("expected %d handler got %d", 1, monitor.numHandlers.Load())
	}
	if !reflect.DeepEqual(handlerRegistration.GetKey(), key) {
		t.Errorf("expected key %v got key %v", key, handlerRegistration.GetKey())
	}
}

func TestRemoveEventHandler(t *testing.T) {
	scenarios := []struct {
		name         string
		isNilHandler bool
		numhandler   int32
		expectErr    bool
	}{
		{
			name:         "nil handler is provided",
			isNilHandler: true,
			numhandler:   1,
			expectErr:    true,
		},
		{
			name:         "correct handler is provided",
			isNilHandler: false,
			numhandler:   0,
			expectErr:    false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset()
			monitor := fakeMonitor(context.TODO(), fakeKubeClient, ObjectKey{})
			handlerRegistration, _ := monitor.AddEventHandler(cache.ResourceEventHandlerFuncs{})
			if s.isNilHandler {
				handlerRegistration = nil
			}

			// for handling nil pointer dereference
			defer func() {
				if err := recover(); err != nil && !s.expectErr {
					t.Errorf("unexpected error %v", err)
				}
				// always check numHandlers
				if monitor.numHandlers.Load() != s.numhandler {
					t.Errorf("expected %d handler got %d", s.numhandler, monitor.numHandlers.Load())
				}
			}()

			gotErr := monitor.RemoveEventHandler(handlerRegistration)
			if gotErr != nil && !s.expectErr {
				t.Errorf("unexpected error %v", gotErr)
			}
			if gotErr == nil && s.expectErr {
				t.Errorf("expecting an error, got nil")
			}
		})
	}
}

func TestScenarios(t *testing.T) {

	scenarios := []struct {
		name       string
		routeName  string
		secretName string
		expected   string
		numErr     int
	}{
		{},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {

		})
	}
}
