package controller

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"gopkg.in/v2/yaml"
)

func LoadFile(path string) (*Config, error) {
	content, err := ioutil.ReadFile(path)

	if err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "failed to read config file")
		}
		glog.Infof("No %s file found.  Will try to figure out defaults", path)
	}

	c, err := Load(string(content))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}
	return c, nil
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
	Domain                string `yaml:"domain,omitempty"`
	Exposer               string `yaml:"exposer"`
	ApiServer             string `yaml:"apiserver,omitempty"`
	NodeIP                string `yaml:"node-ip,omitempty"`
	RouteHost             string `yaml:"route-host,omitempty"`
	RouteUsePath          bool   `yaml:"route-use-path,omitempty"`
	ConsoleURL            string `yaml:"console-url,omitempty"`
	AuthorizePath         string `yaml:"authorize-path,omitempty"`
	ApiServerProtocol     string `yaml:"apiserver-protocol"`
	WatchNamespaces       string `yaml:"watch-namespaces"`
	WatchCurrentNamespace bool   `yaml:"watch-current-namespace"`
	HTTP                  bool   `yaml:"http"`
	TLSAcme               bool   `yaml:"tls-acme"`
	UrlTemplate           string `yaml:"urltemplate,omitempty"`
	// original is the input from which the config was parsed.
	original string
}

var (
	DefaultConfig = Config{}
)

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
