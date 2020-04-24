package exposestrategy

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
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

func NewAutoStrategy(exposer, domain, internalDomain, urltemplate string, nodeIP, routeHost, pathMode string, routeUsePath, http, tlsAcme bool, tlsSecretName string, tlsUseWildcard bool, ingressClass string, client *kubernetes.Clientset, encoder runtime.Encoder) (ExposeStrategy, error) {

	exposer, err := getAutoDefaultExposeRule(client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to automatically get exposer rule.  consider setting 'exposer' type in config.yml")
	}
	log.Infof("Using exposer strategy: %s", exposer)

	// only try to get domain if we need wildcard dns and one wasn't given to us
	if len(domain) == 0 && (strings.EqualFold(ingress, exposer)) {
		domain, err = getAutoDefaultDomain(client)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get a domain")
		}
		log.Infof("Using domain: %s", domain)
	}

	return New(exposer, domain, internalDomain, urltemplate, nodeIP, routeHost, pathMode, routeUsePath, http, tlsAcme, tlsSecretName, tlsUseWildcard, ingressClass, client, encoder)
}

func getAutoDefaultExposeRule(c *kubernetes.Clientset) (string, error) {
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

func getLabelMapAsString(m map[string]string) string {
	b := new(bytes.Buffer)
	for key, value := range m {
		fmt.Fprintf(b, "%s=%s,", key, value)
	}

	s := b.String()
	if len(s) > 0 {
		s = s[:len(s)-1]
	}

	return s
}

func getAutoDefaultDomain(c *kubernetes.Clientset) (string, error) {
	nodes, err := c.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
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
	selector, err := metav1.LabelSelectorAsMap(&metav1.LabelSelector{MatchLabels: map[string]string{"fabric8.io/externalIP": "true"}})
	nodes, err = c.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: getLabelMapAsString(selector)})
	if len(nodes.Items) == 1 {
		node := nodes.Items[0]
		ip, err := getExternalIP(node)
		if err != nil {
			return "", err
		}
		return ip + domainExt, nil
	}

	// look for a stackpoint HA proxy
	pod, _ := c.CoreV1().Pods(stackpointNS).Get(context.TODO(), stackpointHAProxy, metav1.GetOptions{})
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
	return "", errors.New("no known automatic ways to get an external ip to use with nip.  Please configure exposecontroller configmap manually see https://github.com/jenkins-x/exposecontroller#configuration")
}

// copied from k8s.io/kubernetes/master/master.go
func getExternalIP(node corev1.Node) (string, error) {
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
		if addr.Type == corev1.NodeExternalIP {
			return addr.Address, nil
		}
		if fallback == "" && addr.Type == corev1.NodeInternalIP {
			fallback = addr.Address
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", errors.New("no node ExternalIP or InternalIP found")
}
