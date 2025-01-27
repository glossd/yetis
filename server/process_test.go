package server

import (
	"bytes"
	"github.com/glossd/yetis/common"
	"os"
	"os/exec"
	"strings"
	"testing"
)

var sleepConfig = common.DeploymentSpec{
	Name:   "default",
	Cmd:    "sleep 0.01",
	Logdir: "stdout",
}

func TestLaunchProcess_PassEnvVar(t *testing.T) {
	cfg := common.DeploymentSpec{
		Name:   "default",
		Cmd:    "printenv YETIS_FOO",
		Logdir: "stdout",
		Env:    []common.EnvVar{{Name: "YETIS_FOO", Value: "foo"}},
	}
	var buf = &bytes.Buffer{}
	_, err := launchProcessWithOut(cfg, buf, true)
	if err != nil {
		t.Error(err)
	}
	res := strings.TrimSpace(buf.String())
	if res != "foo" {
		t.Errorf("expected foo instead of %s", res)
	}
}

func TestLaunchProcess_PassJsonAsEnvVar(t *testing.T) {
	jsonVal := `{"key": "value"}`
	cfg := common.DeploymentSpec{
		Name:   "default",
		Cmd:    "printenv YETIS_FOO",
		Logdir: "stdout",
		Env:    []common.EnvVar{{Name: "YETIS_FOO", Value: jsonVal}},
	}
	var buf = &bytes.Buffer{}
	_, err := launchProcessWithOut(cfg, buf, true)
	if err != nil {
		t.Error(err)
	}
	res := strings.TrimSpace(buf.String())
	if res != jsonVal {
		t.Errorf("expected foo instead of %s", res)
	}
}

func TestLaunchProcess_PassEnvVarWithSingleQuotes(t *testing.T) {
	envVal := `foo'bar`
	cfg := common.DeploymentSpec{
		Name:   "default",
		Cmd:    "printenv YETIS_FOO",
		Logdir: "stdout",
		Env:    []common.EnvVar{{Name: "YETIS_FOO", Value: envVal}},
	}
	var buf = &bytes.Buffer{}
	_, err := launchProcessWithOut(cfg, buf, true)
	if err != nil {
		t.Error(err)
	}
	res := strings.TrimSpace(buf.String())
	if res != envVal {
		t.Errorf("expected foo instead of %s", res)
	}
}

func TestGetLogCounter(t *testing.T) {
	got := getLogCounter("hello-service", "./logcounter")
	if got != 3 {
		t.Errorf("wrong log counter, expected=%d, got=%d", 3, got)
	}

	got = getLogCounter("noexist", "./logcounter")
	if got != -1 {
		t.Errorf("wrong log counter, expected=%d, got=%d", -1, got)
	}
}

func TestLogRotation(t *testing.T) {
	rm := func() {
		out, err := exec.Command("rm", "-rf", "./logrotation").Output()
		if err != nil {
			t.Fatal("failed to clean dir", string(out), err)
		}
	}
	rm()
	t.Cleanup(rm)
	err := exec.Command("mkdir", "logrotation").Run()
	if err != nil {
		t.Fatal("failed to create dir", err)
	}
	config := common.DeploymentSpec{
		Name:          "hello",
		Cmd:           "echo 'Hello World!'",
		LivenessProbe: common.Probe{TcpSocket: common.TcpSocket{Port: 1234}},
		Logdir:        "./logrotation",
	}
	_, _, err = launchProcess(config, true)
	assert(t, err, nil)
	counter := getLogCounter("hello", "./logrotation")
	assert(t, counter, 0)
	_, _, err = launchProcess(config, true)
	assert(t, err, nil)
	counter = getLogCounter("hello", "./logrotation")
	assert(t, counter, 1)
	_, _, err = launchProcess(config, true)
	assert(t, err, nil)
	counter = getLogCounter("hello", "./logrotation")
	assert(t, counter, 2)
	file, err := os.ReadFile("./logrotation/hello-0.log")
	assert(t, err, nil)
	assert(t, strings.TrimSpace(string(file)), "Hello World!")

}

func TestLaunchNonExistingExecutable(t *testing.T) {
	_, _, err := launchProcess(common.DeploymentSpec{Name: "hello", Cmd: "bogus 4000", Logdir: "stdout"}, false)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func assert[T comparable](t *testing.T, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("got %v, wanted %v", got, want)
	}
}
