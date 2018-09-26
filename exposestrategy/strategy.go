package exposestrategy

import (
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	oclient "github.com/openshift/origin/pkg/client"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

type ExposeStrategy interface {
	Add(svc *api.Service) error
	Remove(svc *api.Service) error
}

type Label struct {
	Key   string
	Value string
}

var (
	ExposeLabel                 = Label{Key: "expose", Value: "true"}
	ExposeAnnotation            = Label{Key: "fabric8.io/expose", Value: "true"}
	InjectAnnotation            = Label{Key: "fabric8.io/inject", Value: "true"}
	ExposeAnnotationKey         = "fabric8.io/exposeUrl"
	ApiServicePathAnnotationKey = "api.service.kubernetes.io/path"
)

func New(exposer, domain, urltemplate, nodeIP, routeHost, pathMode string, routeUsePath, http, tlsAcme bool, tlsSecretName string, client *client.Client, restClientConfig *restclient.Config, encoder runtime.Encoder) (ExposeStrategy, error) {
	switch strings.ToLower(exposer) {
	case "loadbalancer":
		strategy, err := NewLoadBalancerStrategy(client, encoder)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create load balancer expose strategy")
		}
		return strategy, nil
	case "nodeport":
		strategy, err := NewNodePortStrategy(client, encoder, nodeIP)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create node port expose strategy")
		}
		return strategy, nil
	case "ingress":
		glog.Infof("stratagy.New %v", http)
		strategy, err := NewIngressStrategy(client, encoder, domain, http, tlsAcme, tlsSecretName, urltemplate, pathMode)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create ingress expose strategy")
		}
		return strategy, nil
	case "route":
		ocfg := *restClientConfig
		ocfg.APIPath = ""
		ocfg.GroupVersion = nil
		ocfg.NegotiatedSerializer = nil
		oc, _ := oclient.New(&ocfg)
		strategy, err := NewRouteStrategy(client, oc, encoder, domain, routeHost, routeUsePath, http)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create ingress expose strategy")
		}
		return strategy, nil
	case "":
		strategy, err := NewAutoStrategy(exposer, domain, urltemplate, nodeIP, routeHost, pathMode, routeUsePath, http, tlsAcme, tlsSecretName, client, restClientConfig, encoder)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create auto expose strategy")
		}
		return strategy, nil
	default:
		return nil, errors.Errorf("unknown expose strategy '%s', must be one of %v", exposer, []string{"Auto", "Ingress", "Route", "NodePort", "LoadBalancer"})
	}
}
