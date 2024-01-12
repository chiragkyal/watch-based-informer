package monitorv3

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

var ns = "sandbox"

// MockSecretsGetter is a mock for corev1.SecretsGetter interface.
type MockSecretsGetter struct {
	restInterface    rest.Interface
	SecretsInterface corev1.SecretInterface
}

// Secrets returns a fake.SecretInterface that can be used to create, update, and delete secrets.
func (m *MockSecretsGetter) Secrets(namespace string) corev1.SecretInterface {
	// if m.SecretsInterface != nil {
	// 	//return m.SecretsInterface
	// 	return m.SecretsInterface
	// }
	return fake.NewSimpleClientset().CoreV1().Secrets(namespace)

}

// RESTClient returns a RESTClient that is used to perform operations on secrets.
func (m *MockSecretsGetter) RESTClient() corev1.SecretsGetter {
	return m
}

// List calls the List method on the underlying fake.SecretInterface.
func (m *MockSecretsGetter) List(opts metav1.ListOptions) (*v1.SecretList, error) {
	return m.Secrets(ns).List(context.TODO(), opts)
}

// Watch calls the Watch method on the underlying fake.SecretInterface.
func (m *MockSecretsGetter) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return m.Secrets(ns).Watch(context.TODO(), opts)
}

// Verb is a generic method that satisfies the rest.Interface.
func (m *MockSecretsGetter) Verb(verb string) *rest.Request {
	return &rest.Request{}
}

// Get is a generic method that satisfies the rest.Interface.
func (m *MockSecretsGetter) Get() *rest.Request {
	return &rest.Request{}
}

// Post is a generic method that satisfies the rest.Interface.
func (m *MockSecretsGetter) Post() *rest.Request {
	return &rest.Request{}
}

// Put is a generic method that satisfies the rest.Interface.
func (m *MockSecretsGetter) Put() *rest.Request {
	return &rest.Request{}
}

// Delete is a generic method that satisfies the rest.Interface.
func (m *MockSecretsGetter) Delete() *rest.Request {
	return &rest.Request{}
}

// List is a generic method that satisfies the rest.Interface.
// func (m *MockSecretsGetter) List() *rest.Request {
// 	return &rest.Request{}
// }

// // Watch is a generic method that satisfies the rest.Interface.
// func (m *MockSecretsGetter) Watch() *rest.Request {
// 	return &rest.Request{}
// }

func TestYourFunction(t *testing.T) {
	// Create an instance of the mock
	// mockSecretsGetter := &MockSecretsGetter{}
	// //mockSecretsGetter.SecretsInterface.Create()

	// // Set expectations for the mock, if needed
	// // For example, you can set mockSecretsGetter.SecretsInterface to a custom implementation

	// // Inject the mock into your function or code that uses the Kubernetes client
	// // kubeClient := fake.NewSimpleClientset()
	// kubeClient := &kubernetes.Clientset{}
	// kubeClient.CoreV1().RESTClient().(*MockSecretsGetter)
	//RESTClient().(*mock.MockSecretsGetter).SecretsInterface = &customSecretsInterface{} // customize if needed

	// Now, when your code calls kubeClient.CoreV1().RESTClient(), it will get the mock instead

	// Your test code here
}
