package common

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	yaml "sigs.k8s.io/yaml/goyaml.v2"
	"strings"
	"time"
)

type Kind string

const (
	Deployment Kind = "Deployment"
	Service    Kind = "Service"
)

type Config struct {
	path string
	Kind Kind
	Spec Spec
}
type Spec struct {
	Name          string
	Cmd           string
	Workdir       string
	Logdir        string
	LivenessProbe Probe `yaml:"livenessProbe"`
	Env           []EnvVar
	Proxy         Proxy
}

type Probe struct {
	TcpSocket           TcpSocket `yaml:"tcpSocket"`
	InitialDelaySeconds float64   `yaml:"initialDelaySeconds"`
	PeriodSeconds       float64   `yaml:"periodSeconds"`
	FailureThreshold    int       `yaml:"failureThreshold"`
	SuccessThreshold    int       `yaml:"successThreshold"`
}

type TcpSocket struct {
	Port int
}

func (p Probe) InitialDelayDuration() time.Duration {
	return time.Millisecond * time.Duration(p.InitialDelaySeconds*1000)
}

func (p Probe) PeriodDuration() time.Duration {
	return time.Millisecond * time.Duration(p.PeriodSeconds*1000)
}

type EnvVar struct {
	Name  string
	Value string
}

type Proxy struct {
	Port     int
	Strategy DeploymentStrategy
}

type DeploymentStrategy struct {
	Type string
}

func ReadConfigs(path string) ([]Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	cs, err := unmarshal(f)
	if err != nil {
		return nil, err
	}

	err = validate(cs)
	if err != nil {
		return nil, err
	}

	return setDefault(filepath.Dir(path), setEnvVars(cs)), nil
}

func setEnvVars(configs []Config) []Config {
	var newConfigs []Config
	for _, config := range configs {
		var newEnvs []EnvVar
		for _, envVar := range config.Spec.Env {
			if strings.HasPrefix(envVar.Value, "$") && len(envVar.Value) > 1 {
				envVal := os.Getenv(envVar.Value[1:])
				if envVal != "" {
					envVar.Value = envVal
				}
				newEnvs = append(newEnvs, envVar)
			} else {
				newEnvs = append(newEnvs, envVar)
			}
		}
		config.Spec.Env = newEnvs
		newConfigs = append(newConfigs, config)
	}
	return newConfigs
}

func setDefault(defaultPath string, configs []Config) []Config {
	var newConfigs []Config
	for _, config := range configs {
		if config.Kind == "" {
			config.Kind = Deployment
		}
		if config.Spec.Workdir == "" {
			config.Spec.Workdir = defaultPath
		}
		if config.Spec.Logdir == "" {
			config.Spec.Logdir = defaultPath
		}
		if config.Spec.LivenessProbe.InitialDelaySeconds == 0 {
			config.Spec.LivenessProbe.InitialDelaySeconds = 10
		}
		if config.Spec.LivenessProbe.PeriodSeconds == 0 {
			config.Spec.LivenessProbe.PeriodSeconds = 10
		}
		if config.Spec.LivenessProbe.FailureThreshold == 0 {
			config.Spec.LivenessProbe.FailureThreshold = 3
		}
		if config.Spec.LivenessProbe.SuccessThreshold == 0 {
			config.Spec.LivenessProbe.SuccessThreshold = 1
		}

		newConfigs = append(newConfigs, config)
	}
	return newConfigs
}

func validate(configs []Config) error {
	for _, config := range configs {
		if config.Kind != Deployment && config.Kind != Service && config.Kind != "" {
			return fmt.Errorf("invalid kind: allowed Deployment, Service")
		}
		if config.Spec.Cmd == "" {
			return fmt.Errorf("invalid spec: cmd is required")
		}
		if config.Spec.Name == "" {
			return fmt.Errorf("invalid spec: name is required")
		}

		if config.Spec.Proxy.Port == 0 && config.Spec.LivenessProbe.TcpSocket.Port == 0 {
			return fmt.Errorf("proxy is not configured, livenessProbe.tcpSocket.port must be specified")
		}
	}
	return nil
}

func getCurrentExecDir() string {
	ex, err := os.Executable()
	if err != nil {
		log.Fatalf("Could not get executable: %s", err)
	}
	return filepath.Dir(ex)
}

func unmarshal(input io.Reader) ([]Config, error) {
	var configs []Config
	d := yaml.NewDecoder(input)
	for {
		var c Config
		err := d.Decode(&c)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		configs = append(configs, c)
	}

	return configs, nil
}
