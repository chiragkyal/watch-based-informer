package monitorv3

import (
	"context"
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestAddSecretEventHandler(t *testing.T) {
	scenarios := []struct {
		name            string
		routeSecretName string
		expectErr       bool
	}{
		{
			name:            "invalid routeSecretName: r_",
			routeSecretName: "r_",
			expectErr:       true,
		},
		{
			name:            "invalid routeSecretName: _s",
			routeSecretName: "_s",
			expectErr:       true,
		},
		{
			name:            "invalid routeSecretName: rs",
			routeSecretName: "rs",
			expectErr:       true,
		},
		{
			name:            "valid routeSecretName",
			routeSecretName: "r_s",
			expectErr:       false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset()
			key := NewObjectKey("ns", s.routeSecretName)
			sm := secretMonitor{
				kubeClient: fakeKubeClient,
				monitors:   map[ObjectKey]*singleItemMonitor{},
			}

			_, gotErr := sm.AddSecretEventHandler("ns", s.routeSecretName, cache.ResourceEventHandlerFuncs{})
			if gotErr != nil && !s.expectErr {
				t.Errorf("unexpected error %v", gotErr)
			}
			if gotErr == nil && s.expectErr {
				t.Errorf("expecting an error, got nil")
			}
			if !s.expectErr {
				if _, exist := sm.monitors[key]; !exist {
					t.Error("monitor should be added into map", key)
				} else {
					// stop informer to present memory leakage
					sm.monitors[key].StopInformer()
				}
			}
		})
	}
}

func TestRemoveSecretEventHandler(t *testing.T) {
	scenarios := []struct {
		name         string
		isNilHandler bool
		expectErr    bool
	}{
		{
			name:         "nil secret handler is provided",
			isNilHandler: true,
			expectErr:    true,
		},
		{
			name:      "secret handler correctly removed",
			expectErr: false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset()
			key := NewObjectKey("ns", "r_s")
			sm := secretMonitor{
				kubeClient: fakeKubeClient,
				monitors:   map[ObjectKey]*singleItemMonitor{},
			}
			h, err := sm.AddSecretEventHandler(key.Namespace, key.Name, cache.ResourceEventHandlerFuncs{})
			if err != nil {
				t.Error(err)
			}
			if s.isNilHandler {
				// stop informer to present memory leakage
				sm.monitors[key].StopInformer()
				h = nil
			}

			gotErr := sm.RemoveSecretEventHandler(h)
			if gotErr != nil && !s.expectErr {
				t.Errorf("unexpected error %v", gotErr)
			}
			if gotErr == nil && s.expectErr {
				t.Errorf("expecting an error, got nil")
			}
		})
	}
}

func TestGetSecret(t *testing.T) {
	var (
		secretName = "secretName"
	)
	scenarios := []struct {
		name         string
		isNilHandler bool
		expectErr    bool
		expectSecret *v1.Secret
	}{
		// {
		// 	name:         "nil secret handler is provided",
		// 	isNilHandler: true,
		// 	expectErr:    true,
		// 	expectSecret: nil,
		// },
		// {
		// 	name:         "secret does not exist",
		// 	expectErr:    true,
		// 	expectSecret: nil,
		// },
		{
			name:         "correct",
			expectErr:    false,
			expectSecret: fakeSecret("ns", secretName),
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			var fakeKubeClient *fake.Clientset
			key := NewObjectKey("ns", "routeName"+"_"+secretName)
			if s.expectSecret != nil {
				fakeKubeClient = fake.NewSimpleClientset(s.expectSecret)
			}
			sm := secretMonitor{
				kubeClient: fakeKubeClient,
				monitors:   map[ObjectKey]*singleItemMonitor{},
			}
			h, err := sm.AddSecretEventHandler(key.Namespace, key.Name, cache.ResourceEventHandlerFuncs{})
			if err != nil {
				t.Error(err)
			}
			if !cache.WaitForCacheSync(context.TODO().Done(), h.HasSynced) {
				t.Fatal("cache not synced yet")
			}
			if s.isNilHandler {
				// stop informer to present memory leakage
				sm.monitors[key].StopInformer()
				h = nil
			}
			gotSec, gotErr := sm.GetSecret(h)
			if gotErr != nil && !s.expectErr {
				t.Errorf("unexpected error %v", gotErr)
			}
			if gotErr == nil && s.expectErr {
				t.Errorf("expecting an error, got nil")
			}
			if !reflect.DeepEqual(s.expectSecret, gotSec) {
				t.Errorf("expected %v got %v", s.expectSecret, gotSec)
			}
		})
	}
}

// Tomorrow test with origin client
func TestGetSecrets(t *testing.T) {
	ns := "sandbox"
	secretName := "secretName"
	secret := fakeSecret(ns, secretName)
	fakeKubeClient := fake.NewSimpleClientset(secret)
	sm := NewSecretMonitor(fakeKubeClient)

	queue := make(chan string)
	//key := NewObjectKey(ns, "routeName"+"_"+secretName)
	// sm := secretMonitor{
	// 	kubeClient: fakeKubeClient,
	// 	monitors:   map[ObjectKey]*singleItemMonitor{},
	// }
	h, err := sm.AddSecretEventHandler(ns, "routeName"+"_"+secretName, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			queue <- "got it"
		},
	})
	if err != nil || h == nil {
		t.Error(err)
	}
	time.Sleep(time.Second)

	// ch := make(chan struct{})
	// //t.Error(h)
	// if !cache.WaitForCacheSync(ctx.Done(), h.HasSynced) {
	// 	t.Fatal("cache not synced yet")
	// }
	// if err := wait.PollImmediate(10*time.Millisecond, time.Second, func() (done bool, err error) { return h.HasSynced(), nil }); err != nil {
	// 	t.Fatal("cache not synced yet")
	// }
	// wait.PollWithContext()
	// select {
	// case s := <-queue:
	// 	t.Log(s)
	// case <-time.After(10 * time.Second):
	// 	t.Fatal("test timeout")
	// }
	// gotSec, gotErr := sm.GetSecret(h)
	// if gotErr != nil {
	// 	t.Errorf("unexpected error %v", gotErr)
	// }
	// if !reflect.DeepEqual(fakeSecret(ns, secretName), gotSec) {
	// 	t.Errorf("expected %v got %v", fakeSecret(ns, secretName), gotSec)
	// }
	//sm.monitors[key].StopInformer()
}
