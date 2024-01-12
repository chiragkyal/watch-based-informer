package monitorv3

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"io/ioutil"
// 	"net/http"
// 	"reflect"
// 	"testing"

// 	corev1 "k8s.io/api/core/v1"
// 	v1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/client-go/kubernetes"
// 	"k8s.io/client-go/kubernetes/scheme"
// 	restclient "k8s.io/client-go/rest"
// 	fakerest "k8s.io/client-go/rest/fake"
// 	"k8s.io/client-go/tools/cache"
// )

// var (
// 	testNamespace  = "testNamespace"
// 	testSecretName = "testSecretName"
// 	testRouteName  = "testRouteName"
// )

// func fakeSecrett() *corev1.Secret {
// 	return &corev1.Secret{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      testSecretName,
// 			Namespace: testNamespace,
// 		},
// 		Data: map[string][]byte{
// 			"test": {},
// 		},
// 	}
// }

// func SerializeObject(o interface{}) io.ReadCloser {
// 	output, err := json.MarshalIndent(o, "", "")
// 	if err != nil {
// 		panic(err)
// 	}
// 	return ioutil.NopCloser(bytes.NewReader([]byte(output)))
// }

// func GetSecretList() io.ReadCloser {
// 	podList := &v1.SecretList{}
// 	podList.Items = append(podList.Items, *fakeSecret(testNamespace, testSecretName))
// 	podList.TypeMeta = metav1.TypeMeta{
// 		Kind:       "SecretList",
// 		APIVersion: "v1",
// 	}

// 	return SerializeObject(podList)
// }

// func fakeRestClient() *fakerest.RESTClient {
// 	fakeClient := &fakerest.RESTClient{
// 		Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
// 			resp := &http.Response{
// 				StatusCode: http.StatusOK,
// 				Header: map[string][]string{
// 					"Content-Type": {"application/json"},
// 				},
// 				Body: GetSecretList(),
// 			}
// 			return resp, nil
// 		}),
// 		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
// 		GroupVersion:         corev1.SchemeGroupVersion.WithKind("Secret").GroupVersion(),
// 		VersionedAPIPath:     fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", testNamespace, testSecretName),
// 	}
// 	return fakeClient
// }

// func TestMakeMonitorr(t *testing.T) {
// 	fakeClient := fakeRestClient()
// 	conf := restclient.Config{}
// 	fakeKubeClient := kubernetes.NewForConfigOrDie(&conf)
// 	fakeKubeClient.CoreV1().RESTClient().(*restclient.RESTClient).Client = fakeClient.Client
// 	key := NewObjectKey(testNamespace, testRouteName+"_"+testSecretName)
// 	sm := secretMonitor{
// 		kubeClient: fakeKubeClient,
// 		monitors:   map[ObjectKey]*singleItemMonitor{},
// 	}
// 	h, err := sm.AddSecretEventHandler(key.Namespace, key.Name, cache.ResourceEventHandlerFuncs{})
// 	if err != nil || h == nil {
// 		t.Error(err)
// 	}

// 	if !cache.WaitForCacheSync(context.Background().Done(), h.HasSynced) {
// 		t.Fatal("cache not synced yet")
// 	}

// 	gotSec, gotErr := sm.GetSecret(h)
// 	if gotErr != nil {
// 		t.Errorf("unexpected error %v", gotErr)
// 	}
// 	t.Error(gotSec)
// 	if !reflect.DeepEqual(fakeSecret(testNamespace, testSecretName), gotSec) {
// 		t.Errorf("expected %v got %v", fakeSecret(testNamespace, testSecretName), gotSec)
// 	}
// }
