package server

import (
	"bytes"
	"context"
	"github.com/glossd/yetis/common"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var sleepConfig = common.Config{Spec: common.Spec{
	Name:   "default",
	Cmd:    "sleep 0.01",
	Logdir: "stdout",
}}

func TestLaunchProcess_PassEnvVar(t *testing.T) {
	cfg := common.Config{Spec: common.Spec{
		Name:   "default",
		Cmd:    "printenv YETIS_FOO",
		Logdir: "stdout",
		Env:    []common.EnvVar{{Name: "YETIS_FOO", Value: "foo"}},
	}}
	var buf = &bytes.Buffer{}
	_, err := launchProcessWithOut(cfg, buf)
	if err != nil {
		t.Error(err)
	}
	time.Sleep(5 * time.Millisecond)
	res := strings.TrimSpace(buf.String())
	if res != "foo" {
		t.Errorf("expected foo instead of %s", res)
	}
}

func TestIsProcessAlive(t *testing.T) {
	pid, err := launchProcess(sleepConfig)
	if err != nil {
		t.Fatal("launchProcess failed", err)
	}
	if !IsProcessAlive(pid) {
		t.Fatal("process should exist")
	}
	if IsProcessAlive(32768) {
		t.Fatal("pid shouldn't exist") // probs:)
	}

	time.Sleep(11 * time.Millisecond)
	if IsProcessAlive(pid) {
		t.Fatal("sleep should have terminated")
	}
}

func TestTerminateProcess(t *testing.T) {
	c := common.Config{Spec: common.Spec{
		Name:   "default",
		Cmd:    "sleep 10",
		Logdir: "stdout",
	}}
	pid, err := launchProcess(c)
	if err != nil {
		t.Fatalf("error launching process: %s", err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	err = TerminateProcess(ctx, pid)
	if err != nil {
		t.Fatalf("failed to terminated the process: %s", err)
	}
}

func TestIsPortOpen(t *testing.T) {
	s := http.Server{Addr: ":44534"}
	go s.ListenAndServe()
	defer s.Shutdown(context.Background())
	time.Sleep(time.Millisecond)
	if !IsPortOpen(44534, time.Second) {
		t.Errorf("port shouldn't be closed")
	}

	if IsPortOpen(34567, time.Second) {
		t.Errorf("port shouldn't be open")
	}
}

func TestGetPidByPort(t *testing.T) {
	s := http.Server{Addr: ":44534"}
	go s.ListenAndServe()
	defer s.Shutdown(context.Background())

	pid, err := GetPidByPort(44534)
	if err != nil {
		t.Errorf("port is closed")
	}
	if pid == 0 {
		t.Errorf("pid is 0")
	}

	_, err = GetPidByPort(34567)
	if err == nil {
		t.Errorf("port should be closed")
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
	err := exec.Command("mkdir", "logrotation").Run()
	if err != nil {
		t.Fatal("failed to create dir", err)
	}
	config := common.Config{Spec: common.Spec{
		Name:          "hello",
		Cmd:           "echo 'Hello World!'",
		LivenessProbe: common.Probe{TcpSocket: common.TcpSocket{Port: 1234}},
		Logdir:        "./logrotation",
	}}
	_, err = launchProcess(config)
	assert(t, err, nil)
	counter := getLogCounter("hello", "./logrotation")
	assert(t, counter, 0)
	_, err = launchProcess(config)
	assert(t, err, nil)
	counter = getLogCounter("hello", "./logrotation")
	assert(t, counter, 1)
	_, err = launchProcess(config)
	assert(t, err, nil)
	counter = getLogCounter("hello", "./logrotation")
	assert(t, counter, 2)
	time.Sleep(5 * time.Millisecond)
	file, err := os.ReadFile("./logrotation/hello-0.log")
	assert(t, err, nil)
	assert(t, strings.TrimSpace(string(file)), "Hello World!")
	out, err := exec.Command("rm", "-rf", "./logrotation").Output()
	if err != nil {
		t.Fatal("failed to clean dir", string(out), err)
	}
}

func assert[T comparable](t *testing.T, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("got %v, wanted %v", got, want)
	}
}
