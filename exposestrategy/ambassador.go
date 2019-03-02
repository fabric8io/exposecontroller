package exposestrategy

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"gopkg.in/v2/yaml"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// const (
// 	PathModeUsePath = "path"
// )

type AmbassadorStrategy struct {
	client  *client.Client
	encoder runtime.Encoder

	domain        string
	tlsSecretName string
	http          bool
	tlsAcme       bool
	urltemplate   string
	pathMode      string
}

var _ ExposeStrategy = &AmbassadorStrategy{}

func NewAmbassadorStrategy(client *client.Client, encoder runtime.Encoder, domain string, http, tlsAcme bool, urltemplate, pathMode string) (*AmbassadorStrategy, error) {
	glog.Infof("NewAmbassadorStrategy 1 %v", http)
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
	}
	glog.Infof("Using domain: %s", domain)

	var urlformat string
	urlformat, err = getURLFormat(urltemplate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get a url format")
	}
	glog.Infof("Using url template [%s] format [%s]", urltemplate, urlformat)

	return &AmbassadorStrategy{
		client:      client,
		encoder:     encoder,
		domain:      domain,
		http:        http,
		tlsAcme:     tlsAcme,
		urltemplate: urlformat,
		pathMode:    pathMode,
	}, nil
}

func (s *AmbassadorStrategy) Add(svc *api.Service) error {
	appName := svc.Annotations["fabric8.io/ingress.name"]
	if appName == "" {
		if svc.Labels["release"] != "" {
			appName = strings.Replace(svc.Name, svc.Labels["release"]+"-", "", 1)
		} else {
			appName = svc.Name
		}
	}

	hostName := fmt.Sprintf(s.urltemplate, appName, svc.Namespace, s.domain)
	// fullHostName := hostName
	path := svc.Annotations["fabric8.io/ingress.path"]
	pathMode := svc.Annotations["fabric8.io/path.mode"]
	if pathMode == "" {
		pathMode = s.pathMode
	}
	if pathMode == PathModeUsePath {
		suffix := path
		if len(suffix) == 0 {
			suffix = "/"
		}
		path = UrlJoin("/", svc.Namespace, appName, suffix)
		hostName = s.domain
		// fullHostName = UrlJoin(hostName, path)
	}

	exposePort := svc.Annotations[ExposePortAnnotationKey]
	if exposePort != "" {
		port, err := strconv.Atoi(exposePort)
		if err == nil {
			found := false
			for _, p := range svc.Spec.Ports {
				if port == int(p.Port) {
					found = true
					break
				}
			}
			if !found {
				glog.Warningf("Port '%s' provided in the annotation '%s' is not available in the ports of service '%s'",
					exposePort, ExposePortAnnotationKey, svc.GetName())
				exposePort = ""
			}
		} else {
			glog.Warningf("Port '%s' provided in the annotation '%s' is not a valid number",
				exposePort, ExposePortAnnotationKey)
			exposePort = ""
		}
	}
	// Pick the fist port available in the service if no expose port was configured
	if exposePort == "" {
		port := svc.Spec.Ports[0]
		exposePort = strconv.Itoa(int(port.Port))
	}

	servicePort, err := strconv.Atoi(exposePort)
	if err != nil {
		return errors.Wrapf(err, "failed to convert the exposed port '%s' to int", exposePort)
	}
	glog.Infof("Exposing Port %d of Service %s", servicePort, svc.Name)

	// Here's where we start adding the annotations to our service
	prefix := ""
	if path != nil {
		prefix = path
	}
	ambassadorAnnotations := map[string]interface{}{
		"apiVersion": "ambassador/v1",
		"kind":       "Mapping",
		"host":       hostName,
		"prefix":     prefix,
		"name":       fmt.Sprintf("%s_%s_mapping", hostName, svc.Namespace),
		"service":    fmt.Sprintf("%s.%s:%s", appName, svc.Namespace, strconv.Itoa(servicePort))}

	joinedAnnotations := new(bytes.Buffer)
	fmt.Fprintf(joinedAnnotations, "---\n")
	yamlAnnotation, err := yaml.Marshal(&ambassadorAnnotations)
	if err != nil {
		return err
	}
	fmt.Fprintf(joinedAnnotations, "%s", string(yamlAnnotation))

	if s.tlsAcme && svc.Annotations["jenkins-x.io/skip.tls"] != "true" {
		// we need to prepare the tls module config
		ambassadorAnnotations = map[string]interface{}{
			"apiVersion": "ambassador/v1",
			"kind":       "Module",
			"name":       "tls",
			"config": map[string]interface{}{
				"server": map[string]interface{}{
					"enabled": "True",
					"secret":  "tls-" + appName}}}

		yamlAnnotation, err = yaml.Marshal(&ambassadorAnnotations)
		if err != nil {
			return err
		}

		fmt.Fprintf(joinedAnnotations, "---\n")
		fmt.Fprintf(joinedAnnotations, "%s", string(yamlAnnotation))
	}

	svc.Annotations["getambassador.io/config"] = joinedAnnotations.String()

	_, err = s.client.Services(svc.Namespace).Update(svc)
	if err != nil {
		return errors.Wrapf(err, "failed to patch the service %s/%s", svc.Namespace, appName)
	}
	return nil
}

func (s *AmbassadorStrategy) Remove(svc *api.Service) error {
	delete(svc.Annotations, "getambassador.io/config")

	_, err := s.client.Services(svc.Namespace).Update(svc)
	if err != nil {
		return errors.Wrapf(err, "failed to patch the service %s/%s", svc.Namespace, svc.GetName())
	}
	return nil
}
