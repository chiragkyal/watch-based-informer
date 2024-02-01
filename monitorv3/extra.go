package monitorv3

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"testing"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
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

//TestRemoveSecretEventHandler
// if !s.expectErr /*&& !s.isNilHandler && !s.alreadyRemoved */ {
// 	if _, exist := sm.monitors[key]; exist {
// 		sm.monitors[key].StopInformer()
// 		t.Error("monitor key still exists", key)
// 	}
// }

// if s.alreadyRemoved {
// 	//sm.monitors[key].StopInformer()
// 	delete(sm.monitors, key)
// }

// {
// 	name:           "secret monitor already removed",
// 	alreadyRemoved: true,
// 	expectErr:      false,
// },

// if _, exist := sm.monitors[key]; exist {
// 	if !sm.monitors[key].stopped {
// 		sm.monitors[key].StopInformer()
// 	}
// }

// ---------------------------------------------------------------
type blockVerifierFunc func(block *pem.Block) (*pem.Block, error)

func publicKeyBlockVerifier(block *pem.Block) (*pem.Block, error) {
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	block = &pem.Block{
		Type: "PUBLIC KEY",
	}
	if block.Bytes, err = x509.MarshalPKIXPublicKey(key); err != nil {
		return nil, err
	}
	return block, nil
}

func certificateBlockVerifier(block *pem.Block) (*pem.Block, error) {
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	block = &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return block, nil
}

func privateKeyBlockVerifier(block *pem.Block) (*pem.Block, error) {
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			key, err = x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("block %s is not valid", block.Type)
			}
		}
	}
	switch t := key.(type) {
	case *rsa.PrivateKey:
		block = &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(t),
		}
	case *ecdsa.PrivateKey:
		block = &pem.Block{
			Type: "ECDSA PRIVATE KEY",
		}
		if block.Bytes, err = x509.MarshalECPrivateKey(t); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("block private key %T is not valid", key)
	}
	return block, nil
}

func ignoreBlockVerifier(block *pem.Block) (*pem.Block, error) {
	return nil, nil
}

var knownBlockDecoders = map[string]blockVerifierFunc{
	"RSA PRIVATE KEY":   privateKeyBlockVerifier,
	"ECDSA PRIVATE KEY": privateKeyBlockVerifier,
	"PRIVATE KEY":       privateKeyBlockVerifier,
	"PUBLIC KEY":        publicKeyBlockVerifier,
	// Potential "in the wild" PEM encoded blocks that can be normalized
	"RSA PUBLIC KEY":   publicKeyBlockVerifier,
	"DSA PUBLIC KEY":   publicKeyBlockVerifier,
	"ECDSA PUBLIC KEY": publicKeyBlockVerifier,
	"CERTIFICATE":      certificateBlockVerifier,
	// Blocks that should be dropped
	"EC PARAMETERS": ignoreBlockVerifier,
}

// ---------------------------------------------------------------

func TestGetSecrettt(t *testing.T) {
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
			expectSecret: fakeSecret_("ns", secretName),
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
	secret := fakeSecret_(ns, secretName)
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

type testSecretGetterr struct {
	getter    corev1client.SecretsGetter
	secrets   []*corev1.Secret
	namespace string
}

func (t *testSecretGetterr) Secrets(_ string) corev1client.SecretInterface {
	existingObjects := []runtime.Object{}
	for _, s := range t.secrets {
		existingObjects = append(existingObjects, s)
	}
	return fake.NewSimpleClientset(existingObjects...).CoreV1().Secrets(t.namespace)
}

func (t *testSecretGetterr) WithSecret(secret *corev1.Secret) *testSecretGetterr {
	t.secrets = append(t.secrets, secret)
	return t
}

func (t *testSecretGetterr) WithNamespace(namespace string) *testSecretGetterr {
	t.namespace = namespace
	return t
}

func (t *testSecretGetterr) WithSecretData(route *routev1.Route, data map[string]string) *testSecretGetterr {
	t.secrets = append(t.secrets, &corev1.Secret{
		StringData: data,
		Type:       corev1.SecretTypeTLS,
		ObjectMeta: metav1.ObjectMeta{
			Name:      route.Spec.TLS.ExternalCertificate.Name,
			Namespace: route.Namespace,
		},
	})
	return t
}
