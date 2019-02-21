module github.com/jenkins-x/exposecontroller

require (
	cloud.google.com/go v0.0.0-20160916222349-837ecd5c3c75
	github.com/beorn7/perks v0.0.0-20160804104726-4c0e84591b9a
	github.com/blang/semver v3.3.0+incompatible
	github.com/cloudfoundry-incubator/candiedyaml v0.0.0-20160429080125-99c3df83b515
	github.com/coreos/go-oidc v0.0.0-20160829231157-fe7346e2e685
	github.com/coreos/pkg v0.0.0-20160727233714-3ac0863d7acf
	github.com/davecgh/go-spew v0.0.0-20160907170601-6d212800a42e
	github.com/dgrijalva/jwt-go v0.0.0-20150420204307-5ca80149b9d3
	github.com/docker/distribution v0.0.0-20160703061131-559433598c7b
	github.com/docker/docker v0.0.0-20160919125806-c2d6e76a7046
	github.com/docker/engine-api v0.0.0-20160908232104-4290f40c0566
	github.com/docker/go-connections v0.0.0-20160903000609-988efe982fde
	github.com/docker/go-units v0.2.0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
	github.com/emicklei/go-restful v0.0.0-20160919095221-c795848f1d7f
	github.com/evanphx/json-patch v0.0.0-20160803213441-30afec6a1650
	github.com/fsouza/go-dockerclient v0.0.0-20160919115703-bff78c40efd5
	github.com/ghodss/yaml v0.0.0-20160604002925-aa0c86205766
	github.com/gogo/protobuf v0.0.0-20160910082029-a11c89fbb0ad
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20160803200408-a6b377e3400b
	github.com/golang/protobuf v0.0.0-20160829194233-1f49d83d9aa0
	github.com/google/cadvisor v0.23.6
	github.com/google/gofuzz v0.0.0-20160201174807-fd52762d25a4
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v0.0.0-20160902153343-0a192a193177
	github.com/hashicorp/go-cleanhttp v0.0.0-20160407174126-ad28ea4487f0
	github.com/imdario/mergo v0.0.0-20141206190957-6633656539c1
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/jonboulle/clockwork v0.0.0-20160907122059-bcac9884e750
	github.com/juju/ratelimit v0.0.0-20151125201925-77ed1c8a0121
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/opencontainers/runc v0.0.7
	github.com/openshift/origin v1.3.0
	github.com/pborman/uuid v0.0.0-20160824210600-b984ec7fa9ff
	github.com/pkg/errors v0.0.0-20160916110212-a887431f7f6e
	github.com/prometheus/client_golang v0.0.0-20160916180340-5636dc67ae77
	github.com/prometheus/client_model v0.0.0-20150212101744-fa8ad6fec335
	github.com/prometheus/common v0.0.0-20160917184401-9a94032291f2
	github.com/prometheus/procfs v0.0.0-20160411190841-abf152e5f3e9
	github.com/sirupsen/logrus v0.7.3
	github.com/spf13/cobra v0.0.0-20160830174925-9c28e4bbd74e
	github.com/spf13/pflag v0.0.0-20160915153101-c7e63cf4530b
	github.com/ugorji/go v0.0.0-20160911041919-98ef79d6c615
	golang.org/x/net v0.0.0-20160916033756-71a035914f99
	golang.org/x/oauth2 v0.0.0-20160902055913-3c3a985cb79f
	golang.org/x/sys v0.0.0-20160916181909-8f0908ab3b24
	google.golang.org/appengine v0.0.0-20160914034556-78199dcb0669
	gopkg.in/inf.v0 v0.9.0
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/kubernetes v0.0.0-20160901013233-52492b4bff99
)

replace github.com/docker/docker => ./vendor/github.com/docker/docker

replace github.com/docker/distribution => ./vendor/github.com/docker/distribution

replace k8s.io/kubernetes => ./vendor/k8s.io/kubernetes
