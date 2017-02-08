package exposestrategy

import (
    "strings"
	"net/http"
    "io/ioutil"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned/fake"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
    "k8s.io/kubernetes/pkg/client/restclient"
    "k8s.io/kubernetes/pkg/client/unversioned"
)

type MockKubeClient struct {
    RESTClient *fake.RESTClient
    APIClient *testclient.Fake
}

func NewMockKubeClient() MockKubeClient {
    restClient := &fake.RESTClient{
        Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
            httpResp := http.Response {
                StatusCode: 200,
                Body: ioutil.NopCloser(strings.NewReader("Test Response")),
                Request: req,
            }
			return &httpResp, nil
        }),
    }

    fakeClient := testclient.NewSimpleFake()

    return MockKubeClient{
        RESTClient: restClient,
        APIClient: fakeClient,
    }
}

func (c MockKubeClient) Patch(pt api.PatchType) *restclient.Request {
    return c.RESTClient.Patch(pt)
}

func (c MockKubeClient) Get() *restclient.Request {
    return c.RESTClient.Get()
}

func (c MockKubeClient) Ingress(ns string) unversioned.IngressInterface {
    return c.APIClient.Extensions().Ingress(ns)
}

func (c MockKubeClient) Nodes() unversioned.NodeInterface {
    return c.APIClient.Nodes()
}

func (c MockKubeClient) Pods(ns string) unversioned.PodInterface {
    return c.APIClient.Pods(ns)
}
