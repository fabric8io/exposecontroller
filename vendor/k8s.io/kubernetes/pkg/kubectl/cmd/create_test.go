/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"bytes"
	"net/http"
	"testing"

	"k8s.io/kubernetes/pkg/client/unversioned/fake"
)

func TestExtraArgsFail(t *testing.T) {
	initTestErrorHandler(t)
	buf := bytes.NewBuffer([]byte{})

	f, _, _ := NewAPIFactory()
	c := NewCmdCreate(f, buf)
	if ValidateArgs(c, []string{"rc"}) == nil {
		t.Errorf("unexpected non-error")
	}
}

func TestCreateObject(t *testing.T) {
	initTestErrorHandler(t)
	_, _, rc := testData()
	rc.Items[0].Name = "redis-master-controller"

	f, tf, codec := NewAPIFactory()
	tf.Printer = &testPrinter{}
	tf.Client = &fake.RESTClient{
		Codec: codec,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/namespaces/test/replicationcontrollers" && m == "POST":
				return &http.Response{StatusCode: 201, Header: defaultHeader(), Body: objBody(codec, &rc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	tf.Namespace = "test"
	buf := bytes.NewBuffer([]byte{})

	cmd := NewCmdCreate(f, buf)
	cmd.Flags().Set("filename", "../../../examples/guestbook/legacy/redis-master-controller.yaml")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	// uses the name from the file, not the response
	if buf.String() != "replicationcontroller/redis-master-controller\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestCreateMultipleObject(t *testing.T) {
	initTestErrorHandler(t)
	_, svc, rc := testData()

	f, tf, codec := NewAPIFactory()
	tf.Printer = &testPrinter{}
	tf.Client = &fake.RESTClient{
		Codec: codec,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/namespaces/test/services" && m == "POST":
				return &http.Response{StatusCode: 201, Header: defaultHeader(), Body: objBody(codec, &svc.Items[0])}, nil
			case p == "/namespaces/test/replicationcontrollers" && m == "POST":
				return &http.Response{StatusCode: 201, Header: defaultHeader(), Body: objBody(codec, &rc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	tf.Namespace = "test"
	buf := bytes.NewBuffer([]byte{})

	cmd := NewCmdCreate(f, buf)
	cmd.Flags().Set("filename", "../../../examples/guestbook/legacy/redis-master-controller.yaml")
	cmd.Flags().Set("filename", "../../../examples/guestbook/frontend-service.yaml")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	// Names should come from the REST response, NOT the files
	if buf.String() != "replicationcontroller/rc1\nservice/baz\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestCreateDirectory(t *testing.T) {
	initTestErrorHandler(t)
	_, _, rc := testData()
	rc.Items[0].Name = "name"

	f, tf, codec := NewAPIFactory()
	tf.Printer = &testPrinter{}
	tf.Client = &fake.RESTClient{
		Codec: codec,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/namespaces/test/replicationcontrollers" && m == "POST":
				return &http.Response{StatusCode: 201, Header: defaultHeader(), Body: objBody(codec, &rc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	tf.Namespace = "test"
	buf := bytes.NewBuffer([]byte{})

	cmd := NewCmdCreate(f, buf)
	cmd.Flags().Set("filename", "../../../examples/guestbook/legacy")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	if buf.String() != "replicationcontroller/name\nreplicationcontroller/name\nreplicationcontroller/name\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}
