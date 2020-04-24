package exposestrategy

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddServiceAnnotationWithProtocol(t *testing.T) {
	tests := []struct {
		svc                 *corev1.Service
		hostName            string
		protocol            string
		expectedAnnotations map[string]string
	}{
		{
			svc:      &corev1.Service{},
			hostName: "example.com",
			protocol: "http",
			expectedAnnotations: map[string]string{
				ExposeAnnotationKey: "http://example.com",
			},
		},
		{
			svc:      &corev1.Service{},
			hostName: "example.com",
			protocol: "https",
			expectedAnnotations: map[string]string{
				ExposeAnnotationKey: "https://example.com",
			},
		},
		{
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ApiServicePathAnnotationKey: "some/path",
					},
				},
			},
			hostName: "example.com",
			protocol: "https",
			expectedAnnotations: map[string]string{
				ApiServicePathAnnotationKey: "some/path",
				ExposeAnnotationKey:         "https://example.com/some/path",
			},
		},
		{
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ExposeHostNameAsAnnotationKey: "osiris.deislabs.io/ingressHostname",
					},
				},
			},
			hostName: "example.com",
			protocol: "http",
			expectedAnnotations: map[string]string{
				ExposeHostNameAsAnnotationKey:        "osiris.deislabs.io/ingressHostname",
				"osiris.deislabs.io/ingressHostname": "example.com",
				ExposeAnnotationKey:                  "http://example.com",
			},
		},
	}

	for i, test := range tests {
		svc, err := addServiceAnnotationWithProtocol(test.svc, test.hostName, test.protocol)
		if err != nil {
			t.Errorf("[%d] got unexpected error: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(test.expectedAnnotations, svc.Annotations) {
			t.Errorf("[%d] Got the following annotations %#v but expected %#v", i, svc.Annotations, test.expectedAnnotations)
		}
	}
}

func TestRemoveServiceAnnotation(t *testing.T) {
	tests := []struct {
		svc                 *corev1.Service
		expectedAnnotations map[string]string
	}{
		{
			svc:                 &corev1.Service{},
			expectedAnnotations: nil,
		},
		{
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ExposeAnnotationKey: "http://example.com",
						"some-key":          "some value",
					},
				},
			},
			expectedAnnotations: map[string]string{
				"some-key": "some value",
			},
		},
		{
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ExposeHostNameAsAnnotationKey:        "osiris.deislabs.io/ingressHostname",
						"osiris.deislabs.io/ingressHostname": "example.com",
						ApiServicePathAnnotationKey:          "some/path",
						ExposeAnnotationKey:                  "http://example.com/some/path",
					},
				},
			},
			expectedAnnotations: map[string]string{
				ExposeHostNameAsAnnotationKey: "osiris.deislabs.io/ingressHostname",
				ApiServicePathAnnotationKey:   "some/path",
			},
		},
	}

	for i, test := range tests {
		svc := removeServiceAnnotation(test.svc)
		if !reflect.DeepEqual(test.expectedAnnotations, svc.Annotations) {
			t.Errorf("[%d] Got the following annotations %#v but expected %#v", i, svc.Annotations, test.expectedAnnotations)
		}
	}
}
