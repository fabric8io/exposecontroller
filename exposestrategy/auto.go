package exposestrategy

import (
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

const (
	ingress            = "ingress"
	loadBalancer       = "loadbalancer"
	nodePort           = "nodeport"
	route              = "route"
	domainExt          = ".nip.io"
	stackpointNS       = "stackpoint-system"
	stackpointHAProxy  = "spc-balancer"
	stackpointIPEnvVar = "BALANCER_IP"
)

func NewAutoStrategy(exposer, domain, urltemplate string, nodeIP, routeHost string, routeUsePath, http, tlsAcme bool, client *client.Client, restClientConfig *restclient.Config, encoder runtime.Encoder) (ExposeStrategy, error) {

	exposer, err := getAutoDefaultExposeRule(client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to automatically get exposer rule.  consider setting 'exposer' type in config.yml")
	}
	glog.Infof("Using exposer strategy: %s", exposer)

	// only try to get domain if we need wildcard dns and one wasn't given to us
	if len(domain) == 0 && (strings.EqualFold(ingress, exposer)) {
		domain, err = getAutoDefaultDomain(client)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get a domain")
		}
		glog.Infof("Using domain: %s", domain)
	}

	return New(exposer, domain, urltemplate, nodeIP, routeHost, routeUsePath, http, tlsAcme, client, restClientConfig, encoder)
}

func getAutoDefaultExposeRule(c *client.Client) (string, error) {
	t, err := typeOfMaster(c)
	if err != nil {
		return "", errors.Wrap(err, "failed to get type of master")
	}
	if t == openShift {
		return route, nil
	}

	// lets default to Ingress on kubernetes for now
	/*
		nodes, err := c.Nodes().List(api.ListOptions{})
		if err != nil {
			return "", errors.Wrap(err, "failed to find any nodes")
		}
		if len(nodes.Items) == 1 {
			node := nodes.Items[0]
			if node.Name == "minishift" || node.Name == "minikube" {
				return nodePort, nil
			}
		}
	*/
	return ingress, nil
}

func getAutoDefaultDomain(c *client.Client) (string, error) {
	nodes, err := c.Nodes().List(api.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to find any nodes")
	}

	// if we're mini* then there's only one node, any router / ingress controller deployed has to be on this one
	if len(nodes.Items) == 1 {
		node := nodes.Items[0]
		if node.Name == "minishift" || node.Name == "minikube" {
			ip, err := getExternalIP(node)
			if err != nil {
				return "", err
			}
			return ip + domainExt, nil
		}
	}

	// check for a gofabric8 ingress labelled node
	selector, err := unversioned.LabelSelectorAsSelector(&unversioned.LabelSelector{MatchLabels: map[string]string{"fabric8.io/externalIP": "true"}})
	nodes, err = c.Nodes().List(api.ListOptions{LabelSelector: selector})
	if len(nodes.Items) == 1 {
		node := nodes.Items[0]
		ip, err := getExternalIP(node)
		if err != nil {
			return "", err
		}
		return ip + domainExt, nil
	}

	// look for a stackpoint HA proxy
	pod, _ := c.Pods(stackpointNS).Get(stackpointHAProxy)
	if pod != nil {
		containers := pod.Spec.Containers
		for _, container := range containers {
			if container.Name == stackpointHAProxy {
				for _, e := range container.Env {
					if e.Name == stackpointIPEnvVar {
						return e.Value + domainExt, nil
					}
				}
			}
		}
	}
	return "", errors.New("no known automatic ways to get an external ip to use with nip.  Please configure exposecontroller configmap manually see https://github.com/fabric8io/exposecontroller#configuration")
}

// copied from k8s.io/kubernetes/pkg/master/master.go
func getExternalIP(node api.Node) (string, error) {
	var fallback string
	ann := node.Annotations
	if ann != nil {
		for k, v := range ann {
			if len(v) > 0 && strings.HasSuffix(k, "kubernetes.io/provided-node-ip") {
				return v, nil
			}
		}
	}
	for ix := range node.Status.Addresses {
		addr := &node.Status.Addresses[ix]
		if addr.Type == api.NodeExternalIP {
			return addr.Address, nil
		}
		if fallback == "" && addr.Type == api.NodeLegacyHostIP {
			fallback = addr.Address
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", errors.New("no node ExternalIP or LegacyHostIP found")
}
