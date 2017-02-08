package exposestrategy

import (
    "io"
    "fmt"
    "testing"
    "k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
)

type Encoder struct {}

func (e Encoder) Encode(obj runtime.Object, w io.Writer) error {
    return nil
}

func TestAddIngressCreatesExpectedAnnotations(t *testing.T) {
    expectedAnnotation := "kubernetes.io/ingress.class"
    expectedAnnotationValue := "nginx"

    mockClient := NewMockKubeClient()

    encoder := Encoder{}
    mockService := new(api.Service)
    mockService.Name = "test"
    mockService.Namespace = "test"

    ingressStrategy, _ := NewIngressStrategy(mockClient, encoder, "test")
    ingressStrategy.Add(mockService)

    actions := mockClient.APIClient.Actions()
    expectedActions := []string{"get", "create"}
    if len(actions) != len(expectedActions) {
        t.Error(fmt.Sprintf("Expected %d actions, but got %d.", len(expectedActions), len(actions)))
    }

    createAction := actions[1].(testclient.CreateAction)
    createdIngress, ok := createAction.GetObject().(*extensions.Ingress)
    if !ok {
        t.Error("Created object was not an Ingress")
    }

    if createdIngress.Annotations == nil {
        t.Error("No annotations found")
    }

    if ingressClass, ok := createdIngress.Annotations[expectedAnnotation]; ok {
        if ingressClass != expectedAnnotationValue {
            t.Error(fmt.Sprintf("Expected annotation '%s' to have value '%s' but was '%s'", expectedAnnotation,
            expectedAnnotationValue, ingressClass))
        }
    } else {
        t.Error(fmt.Sprintf("Expected annotation '%s' not found", expectedAnnotation))
    }
}
