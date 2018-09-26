package controller

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"gopkg.in/v2/yaml"
)

func LoadFile(path string) (*Config, bool, error) {
	content, err := ioutil.ReadFile(path)

	exists := true
	if err != nil {
		exists = false
		if !os.IsNotExist(err) {
			return nil, exists, errors.Wrap(err, "failed to read config file")
		}
		glog.Infof("No %s file found.  Will try to figure out defaults", path)
	}

	c, err := Load(string(content))
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to read config file")
	}
	return c, exists, nil
}

func Load(s string) (*Config, error) {
	cfg := &Config{}
	// If the entire config body is empty the UnmarshalYAML method is
	// never called. We thus have to set the DefaultConfig at the entry
	// point as well.
	*cfg = DefaultConfig
	err := yaml.Unmarshal([]byte(s), &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	cfg.original = s

	return cfg, nil
}

type Config struct {
	Domain                string   `yaml:"domain,omitempty" json:"domain"`
	Exposer               string   `yaml:"exposer" json:"exposer"`
	PathMode              string   `yaml:"path-mode" json:"path_mode"`
	ApiServer             string   `yaml:"apiserver,omitempty" json:"api_server"`
	NodeIP                string   `yaml:"node-ip,omitempty" json:"node_ip"`
	RouteHost             string   `yaml:"route-host,omitempty" json:"route_host"`
	RouteUsePath          bool     `yaml:"route-use-path,omitempty" json:"route_use_path"`
	ConsoleURL            string   `yaml:"console-url,omitempty" json:"console_url"`
	AuthorizePath         string   `yaml:"authorize-path,omitempty" json:"authorize_path"`
	ApiServerProtocol     string   `yaml:"apiserver-protocol" json:"api_server_protocol"`
	WatchNamespaces       string   `yaml:"watch-namespaces" json:"watch_namespaces"`
	WatchCurrentNamespace bool     `yaml:"watch-current-namespace" json:"watch_current_namespace"`
	HTTP                  bool     `yaml:"http" json:"http"`
	TLSAcme               bool     `yaml:"tls-acme" json:"tls_acme"`
	TLSSecretName         string   `yaml:"tls-secret-name" json:"tls_secret_name"`
	UrlTemplate           string   `yaml:"urltemplate,omitempty" json:"url_template"`
	Services              []string `yaml:"services,omitempty" json:"services"`
	// original is the input from which the config was parsed.
	original string `json:"original"`
}

var (
	DefaultConfig = Config{}
)

// MapToConfig converts the ConfigMap data to a Config object
func MapToConfig(data map[string]string) (*Config, error) {
	answer := &Config{}

	b, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(b, answer)
	return answer, err
}

func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig
	// We want to set c to the defaults and then overwrite it with the input.
	// To make unmarshal fill the plain data struct rather than calling UnmarshalYAML
	// again, we have to hide it using a type indirection.
	type plain Config
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}

	return nil
}

func (c Config) String() string {
	if c.original != "" {
		return c.original
	}

	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	return string(b)
}
