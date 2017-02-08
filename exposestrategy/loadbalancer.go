package exposestrategy

import (
	"reflect"

	"github.com/pkg/errors"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

type LoadBalancerStrategy struct {
	client  KubeClient
	encoder runtime.Encoder
}

var _ ExposeStrategy = &LoadBalancerStrategy{}

func NewLoadBalancerStrategy(client KubeClient, encoder runtime.Encoder) (*LoadBalancerStrategy, error) {
	return &LoadBalancerStrategy{
		client:  client,
		encoder: encoder,
	}, nil
}

func (s *LoadBalancerStrategy) Add(svc *api.Service) error {
	cloned, err := api.Scheme.DeepCopy(svc)
	if err != nil {
		return errors.Wrap(err, "failed to clone service")
	}
	clone, ok := cloned.(*api.Service)
	if !ok {
		return errors.Errorf("cloned to wrong type: %s", reflect.TypeOf(cloned))
	}

	clone.Spec.Type = api.ServiceTypeLoadBalancer
	if len(clone.Spec.LoadBalancerIP) > 0 {
		clone, err = addServiceAnnotation(clone, clone.Spec.LoadBalancerIP)
		if err != nil {
			return errors.Wrap(err, "failed to add service annotation")
		}
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
			return errors.Wrap(err, "failed to send patch")
		}
	}

	return nil
}

func (s *LoadBalancerStrategy) Remove(svc *api.Service) error {
	cloned, err := api.Scheme.DeepCopy(svc)
	if err != nil {
		return errors.Wrap(err, "failed to clone service")
	}
	clone, ok := cloned.(*api.Service)
	if !ok {
		return errors.Errorf("cloned to wrong type: %s", reflect.TypeOf(svc))
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
