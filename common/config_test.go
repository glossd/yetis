package common

import (
	"bytes"
	"testing"
)

func TestConfigUnmarshalSpec(t *testing.T) {
	const fullConfig = `
kind: Deployment
spec:
  name: hello-world # Must be unique
  preCmd: javac HelloWorld.java # Command to execute before the starting the process.  
  cmd: java HelloWorld
  workdir: /home/user/myproject # Directory where command is executed. Defaults to the path in 'apply -f'. 
  logdir: /home/user/myproject/logs # Directory where the logs are stored. Defaults to the path in 'apply -f'.
  strategy:
    type: Recreate # Recreate or RollingUpdate. Defaults to Recreate.
  livenessProbe: # Checks if the command is alive and if not then restarts it
    tcpSocket:
      port: 8080 # Should be specified if proxy is not configured. Defaults to $YETIS_PORT 
    initialDelaySeconds: 5 # Defaults to 10
    periodSeconds: 5 # Defaults to 10
    failureThreshold: 3 # Defaults to 3
    successThreshold: 1 # Defaults to 1
  env: # YETIS_PORT env var is passed by default. You should use alongside proxy config. 
    - name: SOME_SECRET
      value: "pancakes are cakes made in a pan"
    - name: SOME_PASSWORD
      value: mellon
    - name: MY_PORT
      value: $YETIS_PORT # pass the value of the environment variable to another one.
  proxy:
    port: 8080 # Tells linux to forward from the specified port to $YETIS_PORT, allowing zero downtime restarts.
`

	configs, err := unmarshal(bytes.NewBuffer([]byte(fullConfig)))
	if err != nil {
		t.Fatalf("Unmarshal error: %s", err)
	}

	if len(configs) != 1 {
		t.Fatalf("should be one config")
	}
	s := configs[0].Spec.(DeploymentSpec)
	assert(t, s.Cmd, "java HelloWorld")
	assert(t, s.LivenessProbe.PeriodSeconds, 5)
	assert(t, s.Strategy.Type, Recreate)
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
	c := Config{Spec: DeploymentSpec{Env: []EnvVar{{Name: "DIR", Value: "$PWD"}}}}
	newC := setEnvVars([]Config{c})
	ds := newC[0].Spec.(DeploymentSpec)
	if ds.Env[0].Value == "$PWD" {
		t.Fatalf("expected to set env variable")
	}
}

func TestConfigDefault(t *testing.T) {
	c := Config{Spec: DeploymentSpec{}}
	newC := setDefault("./", []Config{c})[0]
	ds := newC.Spec.(DeploymentSpec)
	if ds.LivenessProbe.PeriodSeconds != 10 {
		t.Fatalf("expected to set default value, got=%+v", newC)
	}
}

func assert[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}
