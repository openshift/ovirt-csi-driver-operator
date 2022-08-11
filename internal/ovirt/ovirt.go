package ovirt

import (
	"io/ioutil"
	"os"
	"path/filepath"

	kloglogger "github.com/ovirt/go-ovirt-client-log-klog/v2"
	ovirtclient "github.com/ovirt/go-ovirt-client/v2"
	"gopkg.in/yaml.v2"
)

const defaultOvirtConfigEnvVar = "OVIRT_CONFIG"

// Config holds oVirt api access details.
type Config struct {
	URL      string `yaml:"ovirt_url"`
	Username string `yaml:"ovirt_username"`
	Password string `yaml:"ovirt_password"`
	CAFile   string `yaml:"ovirt_cafile,omitempty"`
	Insecure bool   `yaml:"ovirt_insecure,omitempty"`
}

func NewClient() (ovirtclient.Client, error) {
	ovirtConfig, err := GetOvirtConfig()
	tls := ovirtclient.TLS()
	if ovirtConfig.Insecure {
		tls.Insecure()
	}
	if ovirtConfig.CAFile != "" {
		tls.CACertsFromFile(ovirtConfig.CAFile)
	}
	logger := kloglogger.New()
	//TODO: HANDLE VERBUSE
	client, err := ovirtclient.New(
		ovirtConfig.URL,
		ovirtConfig.Username,
		ovirtConfig.Password,
		tls,
		logger,
		nil,
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// LoadOvirtConfig from the following location (first wins):
// 1. OVIRT_CONFIG env variable
// 2  $defaultOvirtConfigPath
func LoadOvirtConfig() ([]byte, error) {
	data, err := ioutil.ReadFile(discoverPath())
	if err != nil {
		return nil, err
	}
	return data, nil
}

// GetOvirtConfig will return an Config by loading
// the configuration from locations specified in @LoadOvirtConfig
// error is return if the configuration could not be retained.
func GetOvirtConfig() (*Config, error) {
	c := Config{}
	in, err := LoadOvirtConfig()
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(in, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func discoverPath() string {
	path, _ := os.LookupEnv(defaultOvirtConfigEnvVar)
	if path != "" {
		return path
	}
	return filepath.Join(os.Getenv("HOME"), ".ovirt", "ovirt-config.yaml")
}
