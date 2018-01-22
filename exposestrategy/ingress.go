package exposestrategy

import (
	"fmt"
	"reflect"
	"strings"

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

	domain        string
	tlsSecretName string
	http          bool
	tlsAcme       bool
	urltemplate   string
}

var _ ExposeStrategy = &IngressStrategy{}

func NewIngressStrategy(client *client.Client, encoder runtime.Encoder, domain string, http, tlsAcme bool, urltemplate string) (*IngressStrategy, error) {
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

	var urlformat string
	urlformat, err = getURLFormat(urltemplate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get a url format")
	}
	glog.Infof("Using url template [%s] format [%s]", urltemplate, urlformat)

	return &IngressStrategy{
		client:      client,
		encoder:     encoder,
		domain:      domain,
		http:        http,
		tlsAcme:     tlsAcme,
		urltemplate: urlformat,
	}, nil
}

func (s *IngressStrategy) Add(svc *api.Service) error {

	appName := svc.Annotations["fabric8.io/ingress.name"]
	if appName == "" {
		if svc.Labels["release"] != "" {
			appName = strings.Replace(svc.Name, svc.Labels["release"]+"-", "", 1)
		} else {
			appName = svc.Name
		}
	}

	hostName := fmt.Sprintf(s.urltemplate, appName, svc.Namespace, s.domain)

	ingress, err := s.client.Ingress(svc.Namespace).Get(appName)
	createIngress := false
	if err != nil {
		if apierrors.IsNotFound(err) {
			createIngress = true
			ingress = &extensions.Ingress{
				ObjectMeta: api.ObjectMeta{
					Namespace: svc.Namespace,
					Name:      appName,
				},
			}
		} else {
			return errors.Wrapf(err, "could not check for existing ingress %s/%s", svc.Namespace, appName)
		}
	}

	if ingress.Labels == nil {
		ingress.Labels = map[string]string{}
		ingress.Labels["provider"] = "fabric8"
	}

	if ingress.Annotations == nil {
		ingress.Annotations = map[string]string{}
		ingress.Annotations["fabric8.io/generated-by"] = "exposecontroller"
	}

	var tlsSecretName string
	if s.tlsAcme {
		ingress.Annotations["kubernetes.io/tls-acme"] = "true"
		tlsSecretName = "tls-" + appName
	}

	annotationsForIngress := svc.Annotations["fabric8.io/ingress.annotations"]
	if annotationsForIngress != "" {
		annotations := strings.Split(annotationsForIngress, "\n")
		for _, element := range annotations {
			annotation := strings.Split(element, ":")
			key, value := annotation[0], strings.TrimSpace(annotation[1])
			ingress.Annotations[key] = value
		}
	}

	path := svc.Annotations["fabric8.io/ingress.path"]

	backendPaths := []extensions.HTTPIngressPath{}
	if ingress.Spec.Rules != nil {
		backendPaths = ingress.Spec.Rules[0].HTTP.Paths
	}

	// check incase we already have this backend path listed
	for _, path := range backendPaths {
		if path.Backend.ServiceName == svc.Name {
			return nil
		}
	}

	ingress.Spec.Rules = []extensions.IngressRule{}
	for _, port := range svc.Spec.Ports {

		path := extensions.HTTPIngressPath{

			Backend: extensions.IngressBackend{
				ServiceName: svc.Name,
				ServicePort: intstr.FromInt(int(port.Port)),
			},
			Path: path,
		}

		backendPaths = append(backendPaths, path)

		rule := extensions.IngressRule{
			Host: hostName,
			IngressRuleValue: extensions.IngressRuleValue{
				HTTP: &extensions.HTTPIngressRuleValue{
					Paths: backendPaths,
				},
			},
		}

		ingress.Spec.Rules = append(ingress.Spec.Rules, rule)

		if !s.http {
			ingress.Spec.TLS = []extensions.IngressTLS{
				{
					Hosts:      []string{hostName},
					SecretName: tlsSecretName,
				},
			}
		}
	}

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

	if s.http {
		clone, err = addServiceAnnotationWithProtocol(clone, hostName, "http")
	} else {
		clone, err = addServiceAnnotationWithProtocol(clone, hostName, "https")
	}

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
	var appName string
	if svc.Labels["release"] != "" {
		appName = strings.Replace(svc.Name, svc.Labels["release"]+"-", "", 1)
	} else {
		appName = svc.Name
	}
	err := s.client.Ingress(svc.Namespace).Delete(appName, nil)
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
