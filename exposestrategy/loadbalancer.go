package exposestrategy

import (
	"context"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type LoadBalancerStrategy struct {
	client  *kubernetes.Clientset
	encoder runtime.Encoder
}

var _ ExposeStrategy = &LoadBalancerStrategy{}

func NewLoadBalancerStrategy(client *kubernetes.Clientset, encoder runtime.Encoder) (*LoadBalancerStrategy, error) {
	return &LoadBalancerStrategy{
		client:  client,
		encoder: encoder,
	}, nil
}

func (s *LoadBalancerStrategy) Add(svc *corev1.Service) error {
	clone := svc.DeepCopy()

	clone.Spec.Type = corev1.ServiceTypeLoadBalancer
	if len(clone.Spec.LoadBalancerIP) > 0 {
		var err error
		clone, err = addServiceAnnotation(clone, clone.Spec.LoadBalancerIP)
		if err != nil {
			return errors.Wrap(err, "failed to add service annotation")
		}
	}

	patch, err := createPatch(svc, clone, s.encoder, corev1.Service{})
	if err != nil {
		return errors.Wrap(err, "failed to create patch")
	}
	if patch != nil {
		_, err = s.client.CoreV1().Services(svc.Namespace).Patch(context.TODO(), svc.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to send patch")
		}
	}

	return nil
}

func (s *LoadBalancerStrategy) Remove(svc *corev1.Service) error {
	clone := svc.DeepCopy()

	clone = removeServiceAnnotation(clone)

	patch, err := createPatch(svc, clone, s.encoder, corev1.Service{})
	if err != nil {
		return errors.Wrap(err, "failed to create patch")
	}
	if patch != nil {
		_, err = s.client.CoreV1().Services(clone.Namespace).Patch(context.TODO(), clone.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to send patch")
		}
	}

	return nil
}
