# Copyright (C) 2016 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REGISTRY := docker.io/jenkinsxio
GO := GO15VENDOREXPERIMENT=1 go
VERSION := $(shell cat version/VERSION)
OS := $(shell uname)
REVISION=$(shell git rev-parse --short HEAD 2> /dev/null || echo 'unknown')
BRANCH=$(shell git rev-parse --abbrev-ref HEAD 2> /dev/null || echo 'unknown')
HOST=$(shell hostname -f)
BUILD_DATE=$(shell date +%Y%m%d-%H:%M:%S)
GO_VERSION=$(shell go version | sed -e 's/^[^0-9.]*\([0-9.]*\).*/\1/')
PACKAGE_DIRS := $(shell $(GO) list ./... | grep -v /vendor/)
FORMATTED := $(shell $(GO) fmt $(PACKAGE_DIRS))

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_DIR ?= ./out
ORG := github.com/jenkins-x
REPOPATH ?= $(ORG)/exposecontroller
ROOT_PACKAGE := $(shell go list .)

ORIGINAL_GOPATH := $(GOPATH)
GOPATH := $(shell pwd)/_gopath

BUILDFLAGS := -ldflags \
  " -X $(ROOT_PACKAGE)/version.Version='$(VERSION)'\
    -X $(ROOT_PACKAGE)/version.Revision='$(REVISION)'\
    -X $(ROOT_PACKAGE)/version.Branch='$(BRANCH)'\
    -X $(ROOT_PACKAGE)/version.BuildUser='${USER}@$(HOST)'\
    -X $(ROOT_PACKAGE)/version.BuildDate='$(BUILD_DATE)'\
    -X $(ROOT_PACKAGE)/version.GoVersion='$(GO_VERSION)'\
    -s -w -extldflags '-static'"

GOFILES := go list  -f '{{join .Deps "\n"}}' $(REPOPATH) | grep $(REPOPATH) | xargs go list -f '{{ range $$file := .GoFiles }} {{$$.Dir}}/{{$$file}}{{"\n"}}{{end}}'
GOPACKAGES := $(shell go list ./... | grep -v /vendor/)

.PHONY: install
install: $(ORIGINAL_GOPATH)/bin/exposecontroller

fmt:
	@([[ ! -z "$(FORMATTED)" ]] && printf "Fixed unformatted files:\n$(FORMATTED)") || true

$(ORIGINAL_GOPATH)/bin/exposecontroller: out/exposecontroller-$(GOOS)-$(GOARCH)

out/exposecontroller: out/exposecontroller-$(GOOS)-$(GOARCH) fmt

out/exposecontroller-darwin-amd64: $(GOPATH)/src/$(ORG) $(shell $(GOFILES)) version/VERSION
	CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin go build $(BUILDFLAGS) -o $(BUILD_DIR)/exposecontroller-darwin-amd64 $(ROOT_PACKAGE)

out/exposecontroller-linux-amd64: $(GOPATH)/src/$(ORG) $(shell $(GOFILES)) version/VERSION
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build $(BUILDFLAGS) -o $(BUILD_DIR)/exposecontroller-linux-amd64 $(ROOT_PACKAGE)

out/exposecontroller-windows-amd64.exe: $(GOPATH)/src/$(ORG) $(shell $(GOFILES)) version/VERSION
	CGO_ENABLED=0 GOARCH=amd64 GOOS=windows go build $(BUILDFLAGS) -o $(BUILD_DIR)/exposecontroller-windows-amd64.exe $(ROOT_PACKAGE)

out/exposecontroller-linux-arm: $(GOPATH)/src/$(ORG) $(shell $(GOFILES)) version/VERSION
	CGO_ENABLED=0 GOARCH=arm GOOS=linux go build $(BUILDFLAGS) -o $(BUILD_DIR)/exposecontroller-linux-arm $(ROOT_PACKAGE)

.PHONY: test
test: $(GOPATH)/src/$(ORG) out/exposecontroller
	go test -v $(GOPACKAGES)

.PHONY: release
release: clean test cross docker-release

ifeq ($(OS),Darwin)
	sed -i "" -e "s/version:.*/version: $(VERSION)/" charts/exposecontroller/Chart.yaml
	sed -i "" -e "s/ImageTag:.*/ImageTag: $(VERSION)/" charts/exposecontroller/values.yaml

else ifeq ($(OS),Linux)
	sed -i -e "s/version:.*/version: $(VERSION)/" charts/exposecontroller/Chart.yaml
	sed -i -e "s/ImageTag:.*/ImageTag: $(VERSION)/" charts/exposecontroller/values.yaml
else
	exit -1
endif
	git add charts/exposecontroller/Chart.yaml
	git add charts/exposecontroller/values.yaml
	git commit -m "release $(VERSION)" --allow-empty
	mkdir -p release
	cp out/exposecontroller-*-amd64* release
	cp out/exposecontroller-*-arm* release
	gh-release checksums sha256
	gh-release create jenkins-x/exposecontroller $(VERSION) master v$(VERSION)


.PHONY: cross
cross: out/exposecontroller-linux-amd64 out/exposecontroller-darwin-amd64 out/exposecontroller-windows-amd64.exe out/exposecontroller-linux-arm

$(GOPATH)/src/$(ORG):
	mkdir -p $(GOPATH)/src/$(ORG)
	ln -s -f $(shell pwd) $(GOPATH)/src/$(ORG)


.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -rf release

.PHONY: docker
docker: out/exposecontroller-linux-amd64
	docker build -t "$(REGISTRY)/exposecontroller:dev" .

.PHONY: docker-release
docker-release: out/exposecontroller-linux-amd64
	docker build -t $(REGISTRY)/exposecontroller:${VERSION} .
	docker tag $(REGISTRY)/exposecontroller:${VERSION} $(REGISTRY)/exposecontroller:latest
	docker push $(REGISTRY)/exposecontroller:${VERSION}
	docker push $(REGISTRY)/exposecontroller:latest

kube-redeploy: docker
	kubectl delete pod -l project=exposecontroller-app