package exposestrategy

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/intstr"
)

type IngressStrategy struct {
	client  *client.Client
	encoder runtime.Encoder

	domain string
}

var _ ExposeStrategy = &IngressStrategy{}

func NewIngressStrategy(client *client.Client, encoder runtime.Encoder, domain string) (*IngressStrategy, error) {
	t, err := typeOfMaster(client)
	if err != nil {
		return nil, errors.Wrap(err, "could not create new ingress strategy")
	}
	if t == openShift {
		return nil, errors.New("ingress strategy is not supported on OpenShift, please use Route strategy")
	}

	if len(domain) == 0 {
		domain, err = getAutoDefaultDomain(client)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get a domain")
		}
		glog.Infof("Using domain: %s", domain)
	}

	return &IngressStrategy{
		client:  client,
		encoder: encoder,
		domain:  domain,
	}, nil
}

func (s *IngressStrategy) Add(svc *api.Service) error {
	hostName := fmt.Sprintf("%s.%s.%s", svc.Name, svc.Namespace, s.domain)

	ingress, err := s.client.Ingress(svc.Namespace).Get(svc.Name)
	createIngress := false
	if err != nil {
		if apierrors.IsNotFound(err) {
			createIngress = true
			ingress = &extensions.Ingress{
				ObjectMeta: api.ObjectMeta{
					Namespace: svc.Namespace,
					Name:      svc.Name,
				},
			}
		} else {
			return errors.Wrapf(err, "could not check for existing ingress %s/%s", svc.Namespace, svc.Name)
		}
	}

	if ingress.Labels == nil {
		ingress.Labels = map[string]string{}
	}
	ingress.Labels["provider"] = "fabric8"

	if ingress.Annotations == nil {
		ingress.Annotations = map[string]string{}
	}
	ingress.Annotations["kubernetes.io/tls-acme"] = "true"

	ingress.Spec.Rules = []extensions.IngressRule{}
	for _, port := range svc.Spec.Ports {
		rule := extensions.IngressRule{
			Host: hostName,
			IngressRuleValue: extensions.IngressRuleValue{
				HTTP: &extensions.HTTPIngressRuleValue{
					Paths: []extensions.HTTPIngressPath{
						{
							Backend: extensions.IngressBackend{
								ServiceName: svc.Name,
								ServicePort: intstr.FromInt(int(port.Port)),
							},
						},
					},
				},
			},
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
	}

	tls := extensions.IngressTLS{
		Hosts: hostName,
		SecretName: svc.Name + "-tls",
	}
	ingress.Spec.TLS = append(ingress.Spec.TLS, tls)

	if createIngress {
		_, err := s.client.Ingress(ingress.Namespace).Create(ingress)
		if err != nil {
			return errors.Wrapf(err, "failed to create ingress %s/%s", ingress.Namespace, ingress.Name)
		}
	} else {
		_, err := s.client.Ingress(svc.Namespace).Update(ingress)
		if err != nil {
			return errors.Wrapf(err, "failed to update ingress %s/%s", ingress.Namespace, ingress.Name)
		}
	}

	cloned, err := api.Scheme.DeepCopy(svc)
	if err != nil {
		return errors.Wrap(err, "failed to clone service")
	}
	clone, ok := cloned.(*api.Service)
	if !ok {
		return errors.Errorf("cloned to wrong type: %s", reflect.TypeOf(cloned))
	}

	clone, err = addServiceAnnotation(clone, hostName)
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
			return errors.Wrap(err, "failed to send patch")
		}
	}

	return nil
}

func (s *IngressStrategy) Remove(svc *api.Service) error {
	err := s.client.Ingress(svc.Namespace).Delete(svc.Name, nil)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete ingress")
	}

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
