# exposecontroller

Automatically expose services creating ingress rules, openshift routes or modifying services to use kubernetes nodePort or loadBalancer service types

## Getting started

We're adding support for Helm however until then if you want to get going quick using our defaults:

```sh
kubectl create -f http://central.maven.org/maven2/io/fabric8/devops/apps/exposecontroller/2.2.268/exposecontroller-2.2.268-kubernetes.yml
```

```sh
kubectl label svc foo expose=true // now deprecated
```
or
```sh
kubectl annotate svc foo fabric8.io/expose=true
```
If you have [gofabric8](https://github.com/fabric8io/gofabric8) then
```sh
gofabric8 service foo
```

else get the external URL from the service annotation and paste in into your browser
```sh
kubectl get svc foo -o=yaml
```

### Ingress name

The ingress URL uses the service name that contains the `expose` annotation, if you want this to be a different name then annotate the service:
```sh
kubectl annotate svc foo fabric8.io/ingress.name=bar
```

### Multiple backend services

An ingress rule can have multiple service backends, traffic is managed using a path see https://kubernetes.io/docs/concepts/services-networking/ingress/#the-ingress-resource

To add a path to the ingress rule for your service add an annotation:

```sh
kubectl annotate svc foo fabric8.io/ingress.path=/api/
```

### Daemon mode

To run a one shot exposecontroller pass the `--daemon` flag

### Cleanup

To remove any ingress rules created by exposecontroller use the `--cleanup` flag
 
## Configuration

We use a Kubernetes ConfigMap and multiple config entries. Full list [here](https://github.com/fabric8io/exposecontroller/blob/master/controller/config.go#L46)

  - `domain` when using either Kubernetes Ingress or OpenShift Routes you will need to set the domain that you've used with your DNS provider (fabric8 uses [cloudflare](https://www.cloudflare.com)) or nip.io if you want a quick way to get running.
  - `exposer` used to describe which strategy exposecontroller should use to access applications
  - `tls-acme` (boolean) used to enable automatic TLS when used in conjunction with [kube-lego](https://github.com/jetstack/kube-lego). Only works with version v2.3.31 onwards.

### Automatic

If no config map or data values provided exposecontroller will try and work out what `exposer` or `domain` config for the playform.  

* exposer - Minishift anf Minikube will default to `NodePort`, we  use `Ingress` for Kubernetes or `Route` for OpenShift.
* domain - using [nip.io](http://nip.io/) for magic wildcard DNS, exposecontroller will try and find a https://stackpoint.io HAProxy or Nginx Ingress controller.  We also default to the single VM IP if using minishift or minikube.  Together these create an external hostname we can use to access our applications.


### Exposer types
- `Ingress` - [Kubernetes Ingress](http://kubernetes.io/docs/user-guide/ingress/)
- `LoadBalancer` - Cloud provider external [load-balancer](http://kubernetes.io/docs/user-guide/load-balancer/)
- `NodePort` - Recomended for local development using minikube / minishift without Ingress or Router running. See also the [Kubernetes NodePort](http://kubernetes.io/docs/user-guide/services/#type-nodeport) documentation.
- `Route` - OpenShift [Route](https://docs.openshift.com/enterprise/3.2/dev_guide/routes.html)

```yaml
cat <<EOF | kubectl create -f -
apiVersion: "v1"
data:
  config.yml: |-
    exposer: "Ingress"
    domain: "replace.me.io"
kind: "ConfigMap"
metadata:
  name: "exposecontroller"
EOF
```

### OpenShift

Same as above but using the `oc` client binary

___NOTE___ 

If you're using OpenShift then you'll need to add a couple roles:

    oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:default:exposecontroller
    oc adm policy add-cluster-role-to-group cluster-reader system:serviceaccounts # probably too open for all setups

## Label

Now label your service with `expose=true` in [CD Pipelines](https://blog.fabric8.io/create-and-explore-continuous-delivery-pipelines-with-fabric8-and-jenkins-on-openshift-661aa82cb45a#.lx020ys70) or with the CLI:

```sh
kubectl label svc foo expose=true // now deprecated
```
or
```sh
kubectl annotate svc foo fabric8.io/expose=true
```

__exposecontroller__ will use your `exposer` type in the configmap above to automatically watch for new services and create ingress / routes / nodeports / loadbalacers for you.

## Using the expose URL in other resources

Having an external URL is extremely useful. Here are some other uses of the expose URL in addition to the annotation that gets applied to the `Service`

### ConfigMap

Sometimes web applications need to know their external URL so that they can use that link or host/port when generating documentation or links.

For example the [Gogs application](https://github.com/fabric8io/fabric8-devops/tree/master/gogs) needs to know its external URL so that it can show the user how to do a git clone from the command line.

If you wish to enable injection of the expose URL into a `ConfigMap` then 

* create a `ConfigMap` with the same name as the `Service` and in the same namespace
* Add the following annotations to this `ConfigMap` for inserting automatically values into this map when the service gets exposed. The values of these annotations are used as keys in this config map.
  - `expose.config.fabric8.io/url-key` : Exposed URL
  - `expose.config.fabric8.io/host-key` : host or host + port when port is not equal 80 (e.g. `host:port`) 
  - `expose.config.fabric8.io/apiserver-key` : Kubernetes / OpenShift API server host and port (format `host:port`)
  - `expose.config.fabric8.io/apiserver-url-key` : Kubernetes / OpenShift API server URL (format `https://host:port`)
  - `expose.config.fabric8.io/apiserver-protocol-key` : Kubernetes / OpenShift API server protocol (either http or https)
  - `expose.config.fabric8.io/url-protocol` : The default protocol used by Kubernetes Ingress or OpenShift Routes when exposing URLs
  - `expose.config.fabric8.io/console-url-key` : OpenShift Web Console URL
  - `expose.config.fabric8.io/oauth-authorize-url-key` : OAuth Authorization URL
  - `expose.service-key.config.fabric8.io/foo` : Exposed URL of the service called `foo`
  - `expose-full.service-key.config.fabric8.io/foo` : Exposed URL of the service called `foo` ensuring that the URL ends with a `/` character
  - `expose-no-path.service-key.config.fabric8.io//foo` : Exposed URL of the service called `foo` with any Service path removed (so just the protocol and host)
  - `expose-no-protocol.service-key.config.fabric8.io/foo` : Exposed URL of the service called `foo` with the http protocol removed
  - `expose-full-no-protocol.service-key.config.fabric8.io/foo` : Exposed URL of the service called `foo` ensuring that the URL ends with a `/` character and the http protocol removed

E.g. when you set an annotation on the config map `expose.config.fabric8.io/url-key: service.url` then an entry to this config map will be added with the key `service.url` and the value of the exposed service URL when a service of the same name as this configmap gets exposed. 

There is an [example](https://github.com/fabric8io/fabric8-devops/blob/master/gogs/src/main/fabric8/gogs-cm.yml#L27) of the use of these annotations in the Gogs `ConfigMap`

### OAuthClient

When using OpenShift and `OAuthClient` you need to ensure your external URL is added to the `redirectURIs` property in the `OAuthClient`.

If you create your `OAuthClient` in the same namespace with the same name as your `Service` then it will have its expose URL added automatically to the `redirectURIs`

## Building

 * install [go version 1.7.1 or later](https://golang.org/doc/install)
 * type the following:
 * when using minikube or minishift expose the docker daemon to build the __exposecontroller__ image and run inside kubernetes.  e.g  `eval $(minikube docker-env)`

```
git clone git://github.com/fabric8io/exposecontroller.git $GOPATH/src/github.com/fabric8io/exposecontroller
cd $GOPATH/src/github.com/fabric8io/exposecontroller

make
```

### Running locally

Make sure you've got your kube config file set up properly (remember to `oc login` if you're using OpenShift).

    make && ./bin/exposecontroller


### Run on Kubernetes or OpenShift

* build the binary
`make`
* build docker image
`make docker`
* run in kubernetes
`kubectl create -f examples/config-map.yml -f examples/deployment.yml`


## Rapid development on Minikube / Minishift

If you run fabric8 in minikube or minishift then you can get rapid feedback of your code via the following:

on openshift:
 * oc edit dc exposecontroller
on kubernetes:
 * kubectl edit deploy exposecontroller

* replace the `fabric8/exposecontroller:xxxx` image with `fabric8/exposecontroller:dev` and save

Now when developing you can:

* use the `kube-redeploy` make target whenever you want to create a new docker image and redeploy
```
make kube-redeploy
```
if the docker build fails you may need to type this first to point your local shell at the docker daemon inside minishift/minikube:
```
eval $(minishift docker-env)
or
eval $(minikube docker-env)

```

### Developing 

Glide is as the exposecontroller package management

 * install [glide](https://github.com/Masterminds/glide#install)


# Future

On startup it would be good to check if an ingress controller is already running in the cluster, if not create one in an appropriate namespace using a `nodeselector` that chooses a node with a public ip.
