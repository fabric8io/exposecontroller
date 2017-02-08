package exposestrategy

import (
	"fmt"
	"net"
	"reflect"
	"strconv"

	"github.com/pkg/errors"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

type NodePortStrategy struct {
	client  KubeClient
	encoder runtime.Encoder

	nodeIP string
}

var _ ExposeStrategy = &NodePortStrategy{}

const ExternalIPLabel = "fabric8.io/externalIP"

func NewNodePortStrategy(client KubeClient, encoder runtime.Encoder) (*NodePortStrategy, error) {
	l, err := client.Nodes().List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list nodes")
	}

	if len(l.Items) != 1 {
		return nil, errors.Errorf("node port strategy can only be used with single node clusters - found %d nodes", len(l.Items))
	}

	n := l.Items[0]
	ip := n.ObjectMeta.Annotations[ExternalIPLabel]
	if len(ip) == 0 {
		addr, err := getNodeHostIP(n)
		if err != nil {
			return nil, errors.Wrap(err, "cannot discover node IP")
		}
		ip = addr.String()
	}

	return &NodePortStrategy{
		client:  client,
		nodeIP:  ip,
		encoder: encoder,
	}, nil
}

// getNodeHostIP returns the provided node's IP, based on the priority:
// 1. NodeExternalIP
// 2. NodeLegacyHostIP
// 3. NodeInternalIP
func getNodeHostIP(node api.Node) (net.IP, error) {
	addresses := node.Status.Addresses
	addressMap := make(map[api.NodeAddressType][]api.NodeAddress)
	for i := range addresses {
		addressMap[addresses[i].Type] = append(addressMap[addresses[i].Type], addresses[i])
	}
	if addresses, ok := addressMap[api.NodeExternalIP]; ok {
		return net.ParseIP(addresses[0].Address), nil
	}
	if addresses, ok := addressMap[api.NodeLegacyHostIP]; ok {
		return net.ParseIP(addresses[0].Address), nil
	}
	if addresses, ok := addressMap[api.NodeInternalIP]; ok {
		return net.ParseIP(addresses[0].Address), nil
	}
	return nil, fmt.Errorf("host IP unknown; known addresses: %v", addresses)
}

func (s *NodePortStrategy) Add(svc *api.Service) error {
	cloned, err := api.Scheme.DeepCopy(svc)
	if err != nil {
		return errors.Wrap(err, "failed to clone service")
	}
	clone, ok := cloned.(*api.Service)
	if !ok {
		return errors.Errorf("cloned to wrong type: %s", reflect.TypeOf(cloned))
	}

	clone.Spec.Type = api.ServiceTypeNodePort

	if len(svc.Spec.Ports) == 0 {
		return errors.Errorf(
			"service %s/%s has no ports specified. Node port strategy requires a node port",
			svc.Namespace, svc.Name,
		)
	}

	if len(svc.Spec.Ports) > 1 {
		return errors.Errorf(
			"service %s/%s has multiple ports specified (%v). Node port strategy can only be used with single port services",
			svc.Namespace, svc.Name, svc.Spec.Ports,
		)
	}

	port := svc.Spec.Ports[0]
	nodePort := strconv.Itoa(int(port.NodePort))
	hostName := net.JoinHostPort(s.nodeIP, nodePort)
	clone, err = addServiceAnnotation(clone, hostName)
	clone.Spec.ExternalIPs = nil
	if err != nil {
		return errors.Wrap(err, "failed to add service annotation")
	}
	patch, err := createPatch(svc, clone, s.encoder, v1.Service{})
	if err != nil {
		return errors.Wrap(err, "failed to create patch")
	}
	if patch != nil {
		err = s.client.Patch(api.StrategicMergePatchType).
			Resource("services").
			Namespace(svc.Namespace).
			Name(svc.Name).
			Body(patch).Do().Error()
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to send patch for %s/%s patch %s", svc.Namespace, svc.Name, string(patch)))
		}
	}

	return nil
}

func (s *NodePortStrategy) Remove(svc *api.Service) error {
	cloned, err := api.Scheme.DeepCopy(svc)
	if err != nil {
		return errors.Wrap(err, "failed to clone service")
	}
	clone, ok := cloned.(*api.Service)
	if !ok {
		return errors.Errorf("cloned to wrong type: %s", reflect.TypeOf(cloned))
	}

	clone = removeServiceAnnotation(clone)

	patch, err := createPatch(svc, clone, s.encoder, v1.Service{})
	if err != nil {
		return errors.Wrap(err, "failed to create patch")
	}
	if patch != nil {
		err = s.client.Patch(api.StrategicMergePatchType).
			Resource("services").
			Namespace(clone.Namespace).
			Name(clone.Name).
			Body(patch).Do().Error()
		if err != nil {
			return errors.Wrap(err, "failed to send patch")
		}
	}

	return nil
}
