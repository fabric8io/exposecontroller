package exposestrategy

import (
	"k8s.io/kubernetes/pkg/api"
    "k8s.io/kubernetes/pkg/client/restclient"
    "k8s.io/kubernetes/pkg/client/unversioned"
)

type KubeClient interface {
    Patch(pt api.PatchType) *restclient.Request
    Get() *restclient.Request
    Ingress(ns string) unversioned.IngressInterface
    Pods(ns string) unversioned.PodInterface
    Nodes() unversioned.NodeInterface
}
