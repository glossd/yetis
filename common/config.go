package common

import (
	"errors"
	"fmt"
	"github.com/glossd/fetch"
	"io"
	"log"
	"os"
	"path/filepath"
	yaml2 "sigs.k8s.io/yaml"
	yaml "sigs.k8s.io/yaml/goyaml.v2"
	"strconv"
	"strings"
	"time"
)

type Kind string

const (
	Deployment Kind = "Deployment"
)

type StrategyType string

const (
	RollingUpdate StrategyType = "RollingUpdate"
	Recreate      StrategyType = "Recreate"
)

type Spec interface {
	Validate() error
	Kind() Kind
	WithDefaults() Spec
}

type DeploymentSpec struct {
	Name          string
	Cmd           string
	PreCmd        string
	Workdir       string
	Logdir        string
	Strategy      DeploymentStrategy
	LivenessProbe Probe `yaml:"livenessProbe"`
	Env           []EnvVar
	Proxy         Proxy
}

func (ds DeploymentSpec) Validate() error {
	if ds.Cmd == "" {
		return fmt.Errorf("invalid spec: cmd is required")
	}
	if ds.Name == "" {
		return fmt.Errorf("invalid spec: name is required")
	}
	if ds.Strategy.Type != Recreate && ds.Strategy.Type != RollingUpdate {
		return fmt.Errorf("invalid strategy type: %s", ds.Strategy.Type)
	}

	return nil
}

func (ds DeploymentSpec) Kind() Kind {
	return Deployment
}

func (ds DeploymentSpec) WithDefaults() Spec {
	if ds.LivenessProbe.InitialDelaySeconds == 0 {
		ds.LivenessProbe.InitialDelaySeconds = 10
	}
	if ds.LivenessProbe.PeriodSeconds == 0 {
		ds.LivenessProbe.PeriodSeconds = 10
	}
	if ds.LivenessProbe.FailureThreshold == 0 {
		ds.LivenessProbe.FailureThreshold = 3
	}
	if ds.LivenessProbe.SuccessThreshold == 0 {
		ds.LivenessProbe.SuccessThreshold = 1
	}
	if ds.Strategy.Type == "" {
		ds.Strategy.Type = Recreate
	}
	return ds
}

func (ds DeploymentSpec) YetisPort() int {
	port, err := strconv.Atoi(ds.GetEnv("YETIS_PORT"))
	if err != nil {
		return 0
	}
	return port
}

func (ds DeploymentSpec) GetEnv(name string) string {
	for _, envVar := range ds.Env {
		if envVar.Name == name {
			return envVar.Value
		}
	}
	return ""
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

func (p Probe) Port() int {
	return p.TcpSocket.Port
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
type DeploymentStrategy struct {
	Type StrategyType
}

type Proxy struct {
	Port int
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

	cs = setDefault(filepath.Dir(path), setEnvVars(cs))

	var errStr string
	for _, c := range cs {
		err := c.Spec.Validate()
		if err != nil {
			errStr += err.Error() + "\n"
		}
	}
	if errStr != "" {
		return nil, errors.New(errStr)
	}

	return cs, nil
}

func setEnvVars(configs []Config) []Config {
	var newConfigs []Config
	for _, config := range configs {
		switch config.Spec.Kind() {
		case Deployment:
			spec := config.Spec.(DeploymentSpec)
			var newEnvs []EnvVar
			for _, envVar := range spec.Env {
				if strings.HasPrefix(envVar.Value, "$") && len(envVar.Value) > 1 && envVar.Value != "$YETIS_PORT" {
					// $YETIS_PORT is set on the server.
					envVal := os.Getenv(envVar.Value[1:])
					if envVal != "" {
						envVar.Value = envVal
					}
					newEnvs = append(newEnvs, envVar)
				} else {
					newEnvs = append(newEnvs, envVar)
				}
			}
			spec.Env = newEnvs
			config.Spec = spec
		}
		newConfigs = append(newConfigs, config)
	}
	return newConfigs
}

func setDefault(defaultPath string, configs []Config) []Config {
	var newConfigs []Config
	for _, config := range configs {
		switch config.Spec.Kind() {
		case Deployment:
			spec := config.Spec.(DeploymentSpec)
			if spec.Workdir == "" {
				spec.Workdir = defaultPath
			}
			if spec.Logdir == "" {
				spec.Logdir = defaultPath
			}
			config.Spec = spec
		}
		config.Spec = config.Spec.WithDefaults()
		newConfigs = append(newConfigs, config)
	}
	return newConfigs
}
func getCurrentExecDir() string {
	ex, err := os.Executable()
	if err != nil {
		log.Fatalf("Could not get executable: %s", err)
	}
	return filepath.Dir(ex)
}

type Config struct {
	Spec Spec
}

func unmarshal(input io.Reader) ([]Config, error) {
	var configs []Config
	type ReadConfig struct {
		Kind Kind
		Spec any
	}

	d := yaml.NewDecoder(input)
	for {
		var c ReadConfig
		err := d.Decode(&c)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		switch c.Kind {
		case "":
			fallthrough // backward compatibility
		case Deployment:
			spec, err := unmarshalSpec[DeploymentSpec](c.Spec)
			if err != nil {
				return nil, fmt.Errorf("invalid deployment spec: %s", err)
			}
			configs = append(configs, Config{Spec: spec})
		default:
			return nil, fmt.Errorf("invalid kind: %s", c.Kind)
		}
	}

	return configs, nil
}

func unmarshalSpec[T Spec](in any) (T, error) {
	var t T
	yamlBytes, err := yaml.Marshal(in)
	if err != nil {
		return t, err
	}
	jsonBytes, err := yaml2.YAMLToJSONStrict(yamlBytes)
	if err != nil {
		return t, err
	}
	res, err := fetch.Unmarshal[T](string(jsonBytes))
	if err != nil {
		return t, err
	}
	return res, nil

}
