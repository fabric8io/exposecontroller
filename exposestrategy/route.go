package exposestrategy

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	oclient "github.com/openshift/origin/pkg/client"
	rapi "github.com/openshift/origin/pkg/route/api"
	rapiv1 "github.com/openshift/origin/pkg/route/api/v1"
	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

type RouteStrategy struct {
	client  KubeClient
	oclient *oclient.Client
	encoder runtime.Encoder

	domain string
}

var _ ExposeStrategy = &RouteStrategy{}

func NewRouteStrategy(client KubeClient, oclient *oclient.Client, encoder runtime.Encoder, domain string) (*RouteStrategy, error) {
	t, err := typeOfMaster(client)
	if err != nil {
		return nil, errors.Wrap(err, "could not create new route strategy")
	}
	if t == kubernetes {
		return nil, errors.New("route strategy is not supported on Kubernetes, please use Ingress strategy")
	}

	if len(domain) == 0 {
		domain, err = getAutoDefaultDomain(client)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get a domain")
		}
		glog.Infof("Using domain: %s", domain)
	}

	rapi.AddToScheme(api.Scheme)
	rapiv1.AddToScheme(api.Scheme)

	return &RouteStrategy{
		client:  client,
		oclient: oclient,
		encoder: encoder,
		domain:  domain,
	}, nil
}

func (s *RouteStrategy) Add(svc *api.Service) error {
	hostName := fmt.Sprintf("%s.%s.%s", svc.Name, svc.Namespace, s.domain)

	createRoute := false
	route, err := s.oclient.Routes(svc.Namespace).Get(svc.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			createRoute = true
			route = &rapi.Route{
				ObjectMeta: api.ObjectMeta{
					Namespace: svc.Namespace,
					Name:      svc.Name,
				},
			}
		} else {
			return errors.Wrapf(err, "could not check for existing ingress %s/%s", svc.Namespace, svc.Name)
		}
	}

	if route.Labels == nil {
		route.Labels = map[string]string{}
	}
	route.Labels["provider"] = "fabric8"

	route.Spec = rapi.RouteSpec{
		Host: hostName,
		To:   rapi.RouteTargetReference{Name: svc.Name},
	}

	if createRoute {
		_, err := s.oclient.Routes(route.Namespace).Create(route)
		if err != nil {
			return errors.Wrapf(err, "failed to create route %s/%s", route.Namespace, route.Name)
		}
	} else {
		_, err := s.oclient.Routes(route.Namespace).Update(route)
		if err != nil {
			return errors.Wrapf(err, "failed to update route %s/%s", route.Namespace, route.Name)
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

func (s *RouteStrategy) Remove(svc *api.Service) error {
	err := s.oclient.Routes(svc.Namespace).Delete(svc.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete route")
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
