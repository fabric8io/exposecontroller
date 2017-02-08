package exposestrategy

import (
	"bytes"
	"encoding/json"
	"net"

	"github.com/pkg/errors"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/strategicpatch"
)

func addServiceAnnotation(svc *api.Service, hostName string) (*api.Service, error) {
	// default to http
	protocol := "http"

	// if a port is on the hostname check is its a default http / https port
	_, port, err := net.SplitHostPort(hostName)
	if err == nil {
		if port == "443" || port == "8443" {
			protocol = "https"
		} else {
			// check if the service port has a name of https
			for _, port := range svc.Spec.Ports {
				if port.Name == "https" {
					protocol = port.Name
				}
			}
		}
	}
	exposeURL := protocol + "://" + hostName
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	svc.Annotations[ExposeAnnotationKey] = exposeURL

	return svc, nil
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

func typeOfMaster(c KubeClient) (masterType, error) {
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
