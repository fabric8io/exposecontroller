package controller

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/jenkins-x/exposecontroller/exposestrategy"
)

const (
	ExposeConfigURLProtocol                       = "expose.config.fabric8.io/url-protocol"
	ExposeConfigURLKeyAnnotation                  = "expose.config.fabric8.io/url-key"
	ExposeConfigHostKeyAnnotation                 = "expose.config.fabric8.io/host-key"
	ExposeConfigClusterPathKeyAnnotation          = "expose.config.fabric8.io/path-key"
	ExposeConfigClusterIPKeyAnnotation            = "expose.config.fabric8.io/clusterip-key"
	ExposeConfigClusterIPPortKeyAnnotation        = "expose.config.fabric8.io/clusterip-port-key"
	ExposeConfigClusterIPPortIfEmptyKeyAnnotation = "expose.config.fabric8.io/clusterip-port-if-empty-key"
	ExposeConfigApiServerKeyAnnotation            = "expose.config.fabric8.io/apiserver-key"
	ExposeConfigApiServerURLKeyAnnotation         = "expose.config.fabric8.io/apiserver-url-key"
	ExposeConfigConsoleURLKeyAnnotation           = "expose.config.fabric8.io/console-url-key"
	ExposeConfigApiServerProtocolKeyAnnotation    = "expose.config.fabric8.io/apiserver-protocol-key"
	ExposeConfigOAuthAuthorizeURLKeyAnnotation    = "expose.config.fabric8.io/oauth-authorize-url-key"

	ExposeConfigYamlAnnotation = "expose.config.fabric8.io/config-yaml"

	OAuthAuthorizeUrlEnvVar = "OAUTH_AUTHORIZE_URL"

	updateOnChangeAnnotation = "configmap.fabric8.io/update-on-change"
)

type Controller struct {
	client  *kubernetes.Clientset
	encoder runtime.Encoder

	factory informers.SharedInformerFactory
	lister  corelisters.ServiceLister
	synced  cache.InformerSynced

	config *Config

	stopCh chan struct{}
}

func NewController(clientset *kubernetes.Clientset, encoder runtime.Encoder, resyncPeriod time.Duration, namespace string, config *Config) (*Controller, error) {
	log.Infof("NewController %v", config.HTTP)

	c := Controller{
		client:  clientset,
		encoder: encoder,
		stopCh:  make(chan struct{}),
		config:  config,
	}

	strategy, err := exposestrategy.New(config.Exposer, config.Domain, config.InternalDomain, config.UrlTemplate, config.NodeIP, config.RouteHost, config.PathMode, config.RouteUsePath, config.HTTP, config.TLSAcme, config.TLSSecretName, config.TLSUseWildcard, config.IngressClass, clientset, encoder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new strategy")
	}

	if len(config.ApiServerProtocol) == 0 {
		config.ApiServerProtocol = kubernetesServiceProtocol(clientset)
	}

	informerFactory := informers.NewFilteredSharedInformerFactory(clientset, resyncPeriod, namespace, nil)
	informer := informerFactory.Core().V1().Services()

	c.factory = informerFactory

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			if svc.Labels[exposestrategy.ExposeLabel.Key] == exposestrategy.ExposeLabel.Value ||
				svc.Annotations[exposestrategy.ExposeAnnotation.Key] == exposestrategy.ExposeAnnotation.Value ||
				svc.Annotations[exposestrategy.InjectAnnotation.Key] == exposestrategy.InjectAnnotation.Value {
				if !isServiceWhitelisted(svc.GetName(), config) {
					return
				}
				err := strategy.Add(svc)
				if err != nil {
					log.Errorf("Add failed: %v", err)
				}
				updateRelatedResources(clientset, svc, config)
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			svc := newObj.(*corev1.Service)
			if svc.Labels[exposestrategy.ExposeLabel.Key] == exposestrategy.ExposeLabel.Value ||
				svc.Annotations[exposestrategy.ExposeAnnotation.Key] == exposestrategy.ExposeAnnotation.Value ||
				svc.Annotations[exposestrategy.InjectAnnotation.Key] == exposestrategy.InjectAnnotation.Value {
				if !isServiceWhitelisted(svc.GetName(), config) {
					return
				}
				err := strategy.Add(svc)
				if err != nil {
					log.Errorf("Add failed: %v", err)
				}
				updateRelatedResources(clientset, svc, config)
			} else {
				oldSvc := oldObj.(*corev1.Service)
				if oldSvc.Labels[exposestrategy.ExposeLabel.Key] == exposestrategy.ExposeLabel.Value ||
					oldSvc.Annotations[exposestrategy.ExposeAnnotation.Key] == exposestrategy.ExposeAnnotation.Value ||
					svc.Annotations[exposestrategy.InjectAnnotation.Key] == exposestrategy.InjectAnnotation.Value {
					if !isServiceWhitelisted(svc.GetName(), config) {
						return
					}
					err := strategy.Remove(svc)
					if err != nil {
						log.Errorf("Remove failed: %v", err)
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			svc, ok := obj.(cache.DeletedFinalStateUnknown)
			if ok {
				// service key is in the form namespace/name
				split := strings.Split(svc.Key, "/")
				ns := split[0]
				name := split[1]
				if !isServiceWhitelisted(name, config) {
					return
				}
				err := strategy.Remove(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}})
				if err != nil {
					log.Errorf("Remove failed: %v", err)
				}
			}
		},
	})

	c.lister = informer.Lister()
	c.synced = informer.Informer().HasSynced

	return &c, nil
}

// isServiceWhitelisted checks if a service is white-listed in the controller configuration, allow all services if
// the white-list is empty
func isServiceWhitelisted(service string, config *Config) bool {
	services := config.Services
	if len(services) == 0 {
		return true
	}
	for _, s := range services {
		if s == service {
			return true
		}
	}
	return false
}

// findApiServerFromNode lets try default the API server URL by detecting minishift/minikube for single node clusters
func findApiServerFromNode(c *kubernetes.Clientset) string {
	nodes, err := c.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Errorf("Failed to list nodes to detect minishift: %v", err)
		return ""
	}
	items := nodes.Items
	if len(items) != 1 {
		log.Errorf("Number of nodes is %d. We need 1 to detect minishift. Please use  to list nodes to detect minishift: %v", len(items), err)
		return ""
	}
	node := items[0]
	port := "8443"
	ann := node.Annotations
	host := ""
	if ann != nil {
		host = ann["kubernetes.io/hostname"]
	}
	if len(host) == 0 {
		host = node.Spec.DoNotUseExternalID
	}
	if len(host) == 0 {
		host = node.Name
	}
	if len(host) > 0 {
		return host + ":" + port
	}
	return ""

}

func updateRelatedResources(c *kubernetes.Clientset, svc *corev1.Service, config *Config) {
	updateServiceConfigMap(c, svc, config)

	exposeURL := svc.Annotations[exposestrategy.ExposeAnnotationKey]
	if len(exposeURL) > 0 {
		updateOtherConfigMaps(c, svc, config, exposeURL)
	}
}

func kubernetesServiceProtocol(c *kubernetes.Clientset) string {
	hasHttp := false
	svc, err := c.CoreV1().Services("default").Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		log.Warningf("Could not find kubernetes service in the default namespace so we could not detect whether to use http or https as the apiserver protocol. Error: %v", err)
	} else {
		for _, port := range svc.Spec.Ports {
			if port.Name == "https" || port.Port == 443 {
				return "https"
			}
			if port.Name == "http" || port.Port == 80 {
				hasHttp = true
			}
		}
	}
	if hasHttp {
		return "http"
	}
	return "https"
}

func GetServicePort(svc *corev1.Service) string {
	for _, port := range svc.Spec.Ports {
		tp := port.TargetPort.StrVal
		if tp != "" {
			return tp
		}
		i := port.TargetPort.IntVal
		if i > 0 {
			return strconv.Itoa(int(i))
		}
	}
	return ""
}

type ConfigYaml struct {
	Key        string
	Expression string
	Prefix     string
	Suffix     string
}

func updateServiceConfigMap(c *kubernetes.Clientset, svc *corev1.Service, config *Config) {
	name := svc.Name
	ns := svc.Namespace
	cm, err := c.CoreV1().ConfigMaps(ns).Get(context.TODO(), name, metav1.GetOptions{})
	apiserverURL := ""
	apiserver := ""
	apiserverProtocol := ""
	if err == nil {
		updated := false
		apiserver = config.ApiServer
		apiserverProtocol = config.ApiServerProtocol
		consoleURL := config.ConsoleURL

		if len(apiserver) > 0 {
			apiserverURL = apiserverProtocol + "://" + apiserver
			apiServerKey := cm.Annotations[ExposeConfigApiServerKeyAnnotation]
			if len(apiServerKey) > 0 {
				if cm.Data[apiServerKey] != apiserver {
					cm.Data[apiServerKey] = apiserver
					updated = true
				}
			}
			apiServerURLKey := cm.Annotations[ExposeConfigApiServerURLKeyAnnotation]
			if len(apiServerURLKey) > 0 {
				if cm.Data[apiServerURLKey] != apiserverURL {
					cm.Data[apiServerURLKey] = apiserverURL
					updated = true
				}
			}
		}
		if len(consoleURL) > 0 {
			consoleURLKey := cm.Annotations[ExposeConfigConsoleURLKeyAnnotation]
			if len(consoleURLKey) > 0 {
				if cm.Data[consoleURLKey] != consoleURL {
					cm.Data[consoleURLKey] = consoleURL
					updated = true
				}
			}
		}
		apiserverProtocolKey := cm.Annotations[ExposeConfigApiServerProtocolKeyAnnotation]
		if len(apiserverProtocolKey) > 0 {
			if cm.Data[apiserverProtocolKey] != apiserverProtocol {
				cm.Data[apiserverProtocolKey] = apiserverProtocol
				updated = true
			}
		}

		clusterIP := svc.Spec.ClusterIP
		if clusterIP != "" {
			clusterIPKey := firstMapValue(ExposeConfigClusterIPKeyAnnotation, svc.Annotations, cm.Annotations)
			clusterIPPortKey := firstMapValue(ExposeConfigClusterIPPortKeyAnnotation, svc.Annotations, cm.Annotations)
			clusterIPPortIfEmptyKey := firstMapValue(ExposeConfigClusterIPPortIfEmptyKeyAnnotation, svc.Annotations, cm.Annotations)

			if clusterIPKey != "" {
				if cm.Data[clusterIPKey] != clusterIP {
					cm.Data[clusterIPKey] = clusterIP
					updated = true
				}
			}

			port := GetServicePort(svc)
			if port != "" {
				clusterIPAndPort := clusterIP + ":" + port

				if clusterIPPortKey != "" {
					if cm.Data[clusterIPPortKey] != clusterIPAndPort {
						cm.Data[clusterIPPortKey] = clusterIPAndPort
						updated = true
					}
				}
				if clusterIPPortIfEmptyKey != "" {
					if cm.Data[clusterIPPortIfEmptyKey] == "" {
						cm.Data[clusterIPPortIfEmptyKey] = clusterIPAndPort
						updated = true
					}
				}
			}
		}
		exposeURL := svc.Annotations[exposestrategy.ExposeAnnotationKey]
		if len(exposeURL) > 0 {
			host := ""
			url, err := url.Parse(exposeURL)
			if err != nil {
				log.Errorf("Failed to parse expose URL %s for service %s  error: %v", exposeURL, name, err)

			} else {
				host = url.Host
			}
			urlKey := cm.Annotations[ExposeConfigURLKeyAnnotation]
			domainKey := cm.Annotations[ExposeConfigHostKeyAnnotation]
			if len(urlKey) > 0 {
				if cm.Data[urlKey] != exposeURL {
					cm.Data[urlKey] = exposeURL
					updated = true
				}
			}
			if len(host) > 0 && len(domainKey) > 0 {
				if cm.Data[domainKey] != host {
					cm.Data[domainKey] = host
					updated = true
				}
			}

			pathKey := cm.Annotations[ExposeConfigClusterPathKeyAnnotation]
			if pathKey != "" {
				path := urlPath(exposeURL)
				if cm.Data[pathKey] != path {
					cm.Data[pathKey] = path
					updated = true
				}
				log.Infof("Found key %s and has path %s\n", pathKey, path)
			}

			configYaml := svc.Annotations[ExposeConfigYamlAnnotation]
			if configYaml != "" {
				fmt.Printf("Procssing ConfigYaml on service %s\n", svc.Name)
				configs := []ConfigYaml{}
				err := yaml.Unmarshal([]byte(configYaml), &configs)
				if err != nil {
					log.Errorf("Failed to unmarshal Config YAML on service %s due to %s : YAML: %s", svc.Name, err, configYaml)
				} else {
					values := map[string]string{
						"host":              host,
						"url":               exposeURL,
						"apiserver":         apiserver,
						"apiserverURL":      apiserverURL,
						"apiserverProtocol": apiserverProtocol,
						"consoleURL":        consoleURL,
					}
					fmt.Printf("Loading ConfigYaml configurations %#v\n", configs)
					for _, c := range configs {
						if c.UpdateConfigMap(cm, values) {
							updated = true
						}
					}
				}
			}
		}
		if updated {
			log.Infof("Updating ConfigMap %s/%s", ns, name)
			_, err = c.CoreV1().ConfigMaps(ns).Update(context.TODO(), cm, metav1.UpdateOptions{})
			if err != nil {
				log.Errorf("Failed to update ConfigMap %s error: %v", name, err)
			}
			err = rollingUpgradeDeployments(cm, c)
			if err != nil {
				log.Errorf("Failed to update Deployments after change to ConfigMap %s error: %v", name, err)
			}
		}
	}
}

// returns the path starting with a `/` character for the given URL
func urlPath(urlText string) string {
	answer := "/"
	u, err := url.Parse(urlText)
	if err != nil {
		log.Warningf("Could not parse exposeUrl: %s due to: %s", urlText, err)
	} else {
		if u.Path != "" {
			answer = u.Path
		}
		if !strings.HasPrefix(answer, "/") {
			answer = "/" + answer
		}
	}
	return answer
}

// firstMapValue returns the first value in the map which is not empty
func firstMapValue(key string, maps ...map[string]string) string {
	for _, m := range maps {
		if m != nil {
			v := m[key]
			if v != "" {
				return v
			}
		}
	}
	return ""
}

func (c *ConfigYaml) UpdateConfigMap(configMap *corev1.ConfigMap, values map[string]string) bool {
	key := c.Key
	if key == "" {
		log.Warningf("ConfigMap %s does not have a key in ConfigYaml settings %#v\n", configMap.Name, c)
		return false
	}
	expValue := values[c.Expression]
	if expValue == "" {
		log.Warningf("Could not calculate expression %s from the ConfigYaml settings %#v possible values are %v\n", c.Expression, c, values)
		return false
	}
	value := configMap.Data[key]
	if value == "" {
		log.Warningf("ConfigMap %s does not have a key %s when trying to apply the ConfigYaml settings %#v\n", configMap.Name, key, c)
		return false
	}
	lines := strings.Split(value, "\n")
	var buffer bytes.Buffer
	for _, line := range lines {
		if strings.HasPrefix(line, c.Prefix) {
			buffer.WriteString(c.Prefix + expValue + c.Suffix)
		} else {
			buffer.WriteString(line)
		}
		buffer.WriteString("\n")
	}
	newValue := buffer.String()
	if newValue != value {
		configMap.Data[key] = newValue
		return true
	}
	return false
}

func urlJoin(s1 string, s2 string) string {
	return strings.TrimSuffix(s1, "/") + "/" + strings.TrimPrefix(s2, "/")
}

// updateOtherConfigMaps lets update all other configmaps which want to be injected by this svc exposeURL
func updateOtherConfigMaps(c *kubernetes.Clientset, svc *corev1.Service, config *Config, exposeURL string) error {
	serviceName := svc.Name
	annotationKey := "expose.service-key.config.fabric8.io/" + serviceName
	annotationFullKey := "expose-full.service-key.config.fabric8.io/" + serviceName
	annotationNoProtocolKey := "expose-no-protocol.service-key.config.fabric8.io/" + serviceName
	annotationNoPathKey := "expose-no-path.service-key.config.fabric8.io/" + serviceName
	annotationFullNoProtocolKey := "expose-full-no-protocol.service-key.config.fabric8.io/" + serviceName
	ns := svc.Namespace
	cms, err := c.CoreV1().ConfigMaps(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, cm := range cms.Items {
		update := false
		updateKey := cm.Annotations[annotationKey]
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		if len(updateKey) > 0 {
			exposeURL = strings.TrimSuffix(exposeURL, "/")
			keys := strings.Split(updateKey, ",")
			for _, key := range keys {
				value := cm.Data[key]
				if value != exposeURL {
					cm.Data[key] = exposeURL
					log.Infof("Updating ConfigMap %s in namespace %s with key %s", cm.Name, ns, key)
					update = true
				}
			}
		}
		updateKey = cm.Annotations[annotationFullKey]
		if len(updateKey) > 0 {
			if !strings.HasSuffix(exposeURL, "/") {
				exposeURL += "/"
			}
			keys := strings.Split(updateKey, ",")
			for _, key := range keys {
				value := cm.Data[key]
				if value != exposeURL {
					cm.Data[key] = exposeURL
					log.Infof("Updating ConfigMap %s in namespace %s with key %s", cm.Name, ns, key)
					update = true
				}
			}
		}
		updateKey = cm.Annotations[annotationNoPathKey]
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		if len(updateKey) > 0 {
			u, err := url.Parse(exposeURL)
			if err != nil {
				log.Warningf("Failed to parse URL %s due to %s", exposeURL, err)
			} else {
				u.Path = "/"
				noPathURL := u.String()
				keys := strings.Split(updateKey, ",")
				for _, key := range keys {
					value := cm.Data[key]
					if value != noPathURL {
						cm.Data[key] = noPathURL
						log.Infof("Updating ConfigMap %s in namespace %s with key %s", cm.Name, ns, key)
						update = true
					}
				}
			}
		}
		updateKey = cm.Annotations[annotationNoProtocolKey]
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		if len(updateKey) > 0 {
			exposeURL = strings.TrimSuffix(exposeURL, "/")
			exposeURL = strings.TrimPrefix(exposeURL, "http://")
			exposeURL = strings.TrimPrefix(exposeURL, "https://")
			keys := strings.Split(updateKey, ",")
			for _, key := range keys {
				value := cm.Data[key]
				if value != exposeURL {
					cm.Data[key] = exposeURL
					log.Infof("Updating ConfigMap %s in namespace %s with key %s", cm.Name, ns, key)
					update = true
				}
			}
		}
		updateKey = cm.Annotations[annotationFullNoProtocolKey]
		if len(updateKey) > 0 {
			if !strings.HasSuffix(exposeURL, "/") {
				exposeURL += "/"
			}
			exposeURL = strings.TrimPrefix(exposeURL, "http://")
			exposeURL = strings.TrimPrefix(exposeURL, "https://")
			keys := strings.Split(updateKey, ",")
			for _, key := range keys {
				value := cm.Data[key]
				if value != exposeURL {
					cm.Data[key] = exposeURL
					log.Infof("Updating ConfigMap %s in namespace %s with key %s", cm.Name, ns, key)
					update = true
				}
			}
		}
		updateKey = cm.Annotations[ExposeConfigURLProtocol]
		if len(updateKey) > 0 {
			protocol := "https"
			if config.HTTP {
				protocol = "http"
			}
			keys := strings.Split(updateKey, ",")
			for _, key := range keys {
				value := cm.Data[key]
				if value != protocol {
					cm.Data[key] = protocol
					log.Infof("Updating ConfigMap %s in namespace %s with key %s", cm.Name, ns, key)
					update = true
				}
			}
		}
		if update {
			_, err = c.CoreV1().ConfigMaps(ns).Update(context.TODO(), &cm, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("Failed to update ConfigMap %s in namespace %s with key %s due to %v", cm.Name, ns, updateKey, err)
			}
		}
	}
	return nil
}

// Run starts the controller.
func (c *Controller) Run() {
	log.Infof("starting expose controller")

	go c.factory.Start(c.stopCh)

	if ok := cache.WaitForCacheSync(c.stopCh, c.synced); !ok {
		log.Errorf("failed to wait for caches to sync")
		return
	}

	<-c.stopCh
}

func (c *Controller) Stop() {
	log.Infof("stopping expose controller")

	close(c.stopCh)
}

func (c *Controller) Hasrun() bool {
	return c.synced()
}

func serviceListFunc(c *kubernetes.Clientset, ns string) func(metav1.ListOptions) (*corev1.ServiceList, error) {
	return func(opts metav1.ListOptions) (*corev1.ServiceList, error) {
		return c.CoreV1().Services(ns).List(context.TODO(), opts)
	}
}

func serviceWatchFunc(c *kubernetes.Clientset, ns string) func(options metav1.ListOptions) (watch.Interface, error) {
	return func(options metav1.ListOptions) (watch.Interface, error) {
		return c.CoreV1().Services(ns).Watch(context.TODO(), options)
	}
}
