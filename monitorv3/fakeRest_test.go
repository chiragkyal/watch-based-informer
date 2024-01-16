package monitorv3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/tools/cache"
)

var (
	testNamespace  = "testNamespace"
	testSecretName = "testSecretName"
	testRouteName  = "testRouteName"
)

func fakeSecrett() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSecretName,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"test": {},
		},
	}
}

func SerializeObject(o interface{}) io.ReadCloser {
	output, err := json.MarshalIndent(o, "", "")
	if err != nil {
		panic(err)
	}
	return ioutil.NopCloser(bytes.NewReader([]byte(output)))
}

func GetSecretList() io.ReadCloser {
	secretList := &v1.SecretList{}
	secretList.Items = append(secretList.Items, *fakeSecret(testNamespace, testSecretName))
	secretList.TypeMeta = metav1.TypeMeta{
		Kind:       "SecretList",
		APIVersion: "v1",
	}

	return SerializeObject(secretList)
}

func fakeRestClient() *fakerest.RESTClient {
	fakeClient := &fakerest.RESTClient{
		Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header: map[string][]string{
					"Content-Type": {"application/json"},
				},
				Body: GetSecretList(),
			}
			return resp, nil
		}),
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         corev1.SchemeGroupVersion.WithKind("Secret").GroupVersion(),
		VersionedAPIPath:     fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", testNamespace, testSecretName),
	}
	return fakeClient
}

type testRequest struct {
	r *rest.Request
}

func (t *testRequest) Watch(ctx context.Context) (watch.Interface, error) {
	conf := rest.Config{}
	fakeKubeClient := kubernetes.NewForConfigOrDie(&conf)
	return fakeKubeClient.CoreV1().Secrets(testNamespace).Watch(context.Background(), metav1.ListOptions{})
}

func TestMakeMonitorr(t *testing.T) {
	fakeClient := fakeRestClient()
	conf := rest.Config{}
	fakeKubeClient := kubernetes.NewForConfigOrDie(&conf)

	fakeKubeClient.CoreV1().RESTClient().(*rest.RESTClient).Client = fakeClient.Client
	//fakeKubeClient.CoreV1().Secrets(testNamespace).Create(context.TODO(), fakeSecret(testNamespace, testSecretName), metav1.CreateOptions{})
	key := NewObjectKey(testNamespace, testRouteName+"_"+testSecretName)
	sm := secretMonitor{
		kubeClient: fakeKubeClient,
		// restClient: fakeClient,
		monitors: map[ObjectKey]*singleItemMonitor{},
	}
	h, err := sm.AddSecretEventHandler(key.Namespace, key.Name, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			t.Log("Add event")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			t.Log("Update event")
		},
		DeleteFunc: func(obj interface{}) {
			t.Log("Delete event")
		},
	})
	if err != nil || h == nil {
		t.Error(err)
	}
	//event := &v1.Event{}

	if !cache.WaitForCacheSync(context.Background().Done(), h.HasSynced) {
		t.Fatal("cache not synced yet")
	}

	gotSec, gotErr := sm.GetSecret(h)
	if gotErr != nil {
		t.Errorf("unexpected error %v", gotErr)
	}
	//t.Error(gotSec)
	if !reflect.DeepEqual(fakeSecret(testNamespace, testSecretName), gotSec) {
		t.Errorf("expected %v got %v", fakeSecret(testNamespace, testSecretName), gotSec)
	}
	for {
		time.Sleep(2 * time.Second)
	}
}
