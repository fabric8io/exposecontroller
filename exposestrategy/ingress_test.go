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

type AddAnnotationTest struct {
    ResponseObjects []runtime.Object
    ExpectedActions []string
    GetIngress func([]testclient.Action) *extensions.Ingress
}

func TestAddIngressCreatesExpectedAnnotations(t *testing.T) {
    expectedAnnotation := "kubernetes.io/ingress.class"
    expectedAnnotationValue := "nginx"
    encoder := Encoder{}
    mockService := new(api.Service)
    mockService.Name = "testService"
    mockService.Namespace = "testNamespace"

    testCases := getAddIngressAnnotationTestCases(t, mockService)

    for _, testCase := range testCases {
        mockClient := NewMockKubeClient(
            testCase.ResponseObjects...
        )

        ingressStrategy, _ := NewIngressStrategy(mockClient, encoder, "test")
        ingressStrategy.Add(mockService)

        actions := mockClient.APIClient.Actions()
        if len(actions) != len(testCase.ExpectedActions) {
            t.Error(fmt.Sprintf("Expected %d actions, but got %d.", len(testCase.ExpectedActions), len(actions)))
        }

        for index, actionName := range testCase.ExpectedActions {
            if actions[index].GetVerb() != actionName {
                t.Error(fmt.Sprintf("Expected action %d to be %s but was %s", index, actionName, actions[index].GetVerb()))
            }
        }

        ingress := testCase.GetIngress(actions)
        if ingress.Annotations == nil {
            t.Error("No annotations found")
        }

        if ingressClass, ok := ingress.Annotations[expectedAnnotation]; ok {
            if ingressClass != expectedAnnotationValue {
                t.Error(fmt.Sprintf("Expected annotation '%s' to have value '%s' but was '%s'", expectedAnnotation,
                expectedAnnotationValue, ingressClass))
            }
        } else {
            t.Error(fmt.Sprintf("Expected annotation '%s' not found", expectedAnnotation))
        }
    }
}

func getAddIngressAnnotationTestCases(t *testing.T, mockService *api.Service) []AddAnnotationTest {
    return []AddAnnotationTest{
        {
            ResponseObjects: nil,
            ExpectedActions: []string{"get", "create"},
            GetIngress: func(actions []testclient.Action) *extensions.Ingress {
                createAction := actions[1].(testclient.CreateAction)
                createdIngress, ok := createAction.GetObject().(*extensions.Ingress)
                if !ok {
                    t.Error("Created object was not an Ingress")
                }
                return createdIngress
            },
        },
        {
            ResponseObjects: []runtime.Object{
                &extensions.Ingress{ObjectMeta: api.ObjectMeta{
                    Name: mockService.Name,
                    Namespace: mockService.Namespace,
                }},
            },
            ExpectedActions: []string{"get", "update"},
            GetIngress: func(actions []testclient.Action) *extensions.Ingress {
                updateAction := actions[1].(testclient.UpdateAction)
                updatedIngress, ok := updateAction.GetObject().(*extensions.Ingress)
                if !ok {
                    t.Error("Created object was not an Ingress")
                }
                return updatedIngress
            },
        },
    }
}
