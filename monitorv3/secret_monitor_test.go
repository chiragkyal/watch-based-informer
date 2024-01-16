package monitorv3

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/tools/cache"
)

type testSecretGetter struct {
	restClient        *fakerest.RESTClient
	secrets           []corev1.Secret
	secretsSerialized io.ReadCloser
}

func (t *testSecretGetter) WithSecret(secret corev1.Secret) *testSecretGetter {
	t.secrets = append(t.secrets, secret)
	return t
}

func (t *testSecretGetter) WithSerialized() *testSecretGetter {
	secretList := &v1.SecretList{}
	secretList.TypeMeta = metav1.TypeMeta{
		Kind:       "SecretList",
		APIVersion: "v1",
	}
	secretList.Items = t.secrets
	t.secretsSerialized = serializeObject(secretList)
	return t
}

func (t *testSecretGetter) WithRestClient() *testSecretGetter {
	fakeClient := &fakerest.RESTClient{
		Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header: map[string][]string{
					"Content-Type": {"application/json"},
				},
				Body: t.secretsSerialized,
			}
			return resp, nil
		}),
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         corev1.SchemeGroupVersion.WithKind("Secret").GroupVersion(),
		//VersionedAPIPath:     fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", testNamespace, testSecretName),
	}
	t.restClient = fakeClient
	return t
}

func serializeObject(o interface{}) io.ReadCloser {
	output, err := json.MarshalIndent(o, "", "")
	if err != nil {
		panic(err)
	}
	return ioutil.NopCloser(bytes.NewReader([]byte(output)))
}

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
		testNamespace  = "testNamespace"
		testSecretName = "testSecretName"
		testRouteName  = "testRouteName"
	)

	conf := rest.Config{}
	fakeKubeClient := kubernetes.NewForConfigOrDie(&conf)

	tsg := testSecretGetter{}
	secret := fakeSecret(testNamespace, testSecretName)
	tsg.WithSecret(*secret).WithSerialized()
	tsg.WithRestClient()

	fakeKubeClient.CoreV1().RESTClient().(*rest.RESTClient).Client = tsg.restClient.Client
	key := NewObjectKey(testNamespace, testRouteName+"_"+testSecretName)
	sm := secretMonitor{
		kubeClient: fakeKubeClient,
		monitors:   map[ObjectKey]*singleItemMonitor{},
	}
	h, err := sm.AddSecretEventHandler(key.Namespace, key.Name, cache.ResourceEventHandlerFuncs{})
	if err != nil || h == nil {
		t.Error(err)
	}
	if !cache.WaitForCacheSync(context.Background().Done(), h.HasSynced) {
		t.Fatal("cache not synced yet")
	}

	time.Sleep(2 * time.Second)

	gotSec, gotErr := sm.GetSecret(h)
	if gotErr != nil {
		t.Errorf("unexpected error %v", gotErr)
	}

	if !reflect.DeepEqual(secret, gotSec) {
		t.Errorf("expected %v got %v", secret, gotSec)
	}

	// for testing
	t.Error("test", gotSec)

}
