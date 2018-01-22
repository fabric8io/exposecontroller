package exposestrategy

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/strategicpatch"

	client "k8s.io/kubernetes/pkg/client/unversioned"
)

func findHttpProtocol(svc *api.Service, hostName string) string {
	// default to http
	protocol := "http"

	// if a port is on the hostname check is its a default http / https port
	_, port, err := net.SplitHostPort(hostName)
	if err == nil {
		if port == "443" || port == "8443" {
			protocol = "https"
		}
	}
	// check if the service port has a name of https
	for _, port := range svc.Spec.Ports {
		if port.Name == "https" {
			protocol = port.Name
		}
	}
	return protocol
}

func addServiceAnnotation(svc *api.Service, hostName string) (*api.Service, error) {
	protocol := findHttpProtocol(svc, hostName)
	return addServiceAnnotationWithProtocol(svc, hostName, protocol)
}

func addServiceAnnotationWithProtocol(svc *api.Service, hostName string, protocol string) (*api.Service, error) {
	exposeURL := protocol + "://" + hostName
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	path := svc.Annotations[ApiServicePathAnnotationKey]
	if len(path) > 0 {
		exposeURL = urlJoin(exposeURL, path)
	}
	svc.Annotations[ExposeAnnotationKey] = exposeURL

	return svc, nil
}

// urlJoin joins the given URL paths so that there is a / separating them but not a double //
func urlJoin(repo string, path string) string {
	return strings.TrimSuffix(repo, "/") + "/" + strings.TrimPrefix(path, "/")
}

func removeServiceAnnotation(svc *api.Service) *api.Service {
	delete(svc.Annotations, ExposeAnnotationKey)

	return svc
}

func createPatch(a runtime.Object, b runtime.Object, encoder runtime.Encoder, dataStruct interface{}) ([]byte, error) {
	var aBuf bytes.Buffer
	err := encoder.Encode(a, &aBuf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode object: %v", a)
	}
	var bBuf bytes.Buffer
	err = encoder.Encode(b, &bBuf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode object: %v", b)
	}

	aBytes := aBuf.Bytes()
	bBytes := bBuf.Bytes()

	if bytes.Compare(aBytes, bBytes) == 0 {
		return nil, nil
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(aBytes, bBytes, dataStruct)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create patch")
	}

	return patch, nil
}

type masterType string

const (
	openShift  masterType = "OpenShift"
	kubernetes masterType = "Kubernetes"
)

func typeOfMaster(c *client.Client) (masterType, error) {
	res, err := c.Get().AbsPath("").DoRaw()
	if err != nil {
		errors.Wrap(err, "could not discover the type of your installation")
	}

	var rp unversioned.RootPaths
	err = json.Unmarshal(res, &rp)
	if err != nil {
		errors.Wrap(err, "could not discover the type of your installation")
	}
	for _, p := range rp.Paths {
		if p == "/oapi" {
			return openShift, nil
		}
	}
	return kubernetes, nil
}

type urlTemplateParts struct {
	Service   string
	Namespace string
	Domain    string
}

func getURLFormat(urltemplate string) (string, error) {
	if urltemplate == "" {
		urltemplate = "{{.Service}}.{{.Namespace}}.{{.Domain}}"
	}
	placeholders := urlTemplateParts{"%[1]s", "%[2]s", "%[3]s"}
	tmpl, err := template.New("format").Parse(urltemplate)
	if err != nil {
		errors.Wrap(err, "Failed to parse UrlTemplate")
	}
	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, placeholders)
	if err != nil {
		errors.Wrap(err, "Failed to execute UrlTemplate")
	}
	return buffer.String(), nil
}
