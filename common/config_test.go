package common

import (
	"bytes"
	"testing"
)

func TestConfigUnmarshalSpec(t *testing.T) {
	const fullConfig = `
kind: Deployment
spec:
  name: hello-world # Must be unique per spec
  cmd: java HelloWorld # If lb is enabled, start it on a port from YETIS_PORT env var.
  workdir: $HOME/myproject # Directory where command is executed. Defaults to current. 
  logdir: $HOME/myproject/logs # Directory where the logs are stored. Defaults to current.
  livenessProbe: # Checks if the command is alive on the YETIS_PORT, and if not then restarts it
    tcpSocket:
      port: 8080 # Ignored if proxy is configured. 
    initialDelaySeconds: 5 # Defaults to 0
    periodSeconds: 5 # Defaults to 10
    failureThreshold: 3 # Defaults to 3
    successThreshold: 1 # Defaults to 1
  env:
    - name: SOME_SECRET
      value: "pancakes are cakes made in a pan"
    - name: SOME_PASSWORD
      value: mellon
    - name: MY_PORT
      value: $YETIS_PORT # pass the value of the environment variable to another one. 
  proxy: # WIP: read the Proxy section
    port: 4567 # The port for the sidecar proxy to run. Your 'cmd' must start on YETIS_PORT env var.
    strategy:
      type: RollingUpdate # RollingUpdate or Recreate. Defaults to RollingUpdate.
`

	configs, err := unmarshal(bytes.NewBuffer([]byte(fullConfig)))
	if err != nil {
		t.Fatalf("Unmarshal error: %s", err)
	}

	if len(configs) != 1 {
		t.Fatalf("should be one config")
	}
	assert(t, configs[0].Kind, Deployment)
	s := configs[0].Spec
	assert(t, s.Cmd, "java HelloWorld")
	assert(t, s.LivenessProbe.PeriodSeconds, 5)
	assert(t, s.Proxy.Port, 4567)
	assert(t, s.Proxy.Strategy.Type, "RollingUpdate")
	assert(t, len(s.Env), 3)
	assert(t, s.Env[0].Name, "SOME_SECRET")
	assert(t, s.Env[0].Value, "pancakes are cakes made in a pan")
}

func TestMultipleSpec(t *testing.T) {
	const c = `
spec:
  cmd: npm start
---
spec:
  cmd: npm start
`
	configs, err := unmarshal(bytes.NewBuffer([]byte(c)))
	if err != nil {
		t.Fatalf("Unmarshal error: %s", err)
	}

	if len(configs) != 2 {
		t.Fatalf("failed to unmarshal")
	}
}

func TestConfigSetEnv(t *testing.T) {
	c := Config{Spec: Spec{Env: []EnvVar{{Name: "DIR", Value: "$PWD"}}}}
	newC := setEnvVars([]Config{c})
	if newC[0].Spec.Env[0].Value == "$PWD" {
		t.Fatalf("expected to set env variable")
	}
}

func TestConfigDefault(t *testing.T) {
	c := Config{}
	newC := setDefault("./", []Config{c})[0]
	if newC.Spec.LivenessProbe.PeriodSeconds != 10 || newC.Kind != Deployment {
		t.Fatalf("expected to set default value, got=%+v", newC)
	}
}

func TestConfig(t *testing.T) {
	c := Config{}
	newC := setDefault("./", []Config{c})
	if newC[0].Spec.LivenessProbe.PeriodSeconds != 10 {
		t.Fatalf("expected to set default value")
	}
}

func assert[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}
