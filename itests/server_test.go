package itests

import (
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/client"
	_ "github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"github.com/glossd/yetis/server"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestRestart(t *testing.T) {
	os.Setenv("YETIS_SERVER_LOGDIR", "stdout")
	go server.Start()
	defer server.Stop()
	// let the server start
	time.Sleep(time.Millisecond)
	applyNC(t)
	defer unix.KillByPort(27000)

	check := func(f func(server.GetResponse)) {
		dr, err := fetch.Get[server.GetResponse]("/hello")
		if err != nil {
			t.Fatal(err)
		}
		f(dr)
	}

	checkSR := func(description string, s server.DeploymentStatus, restarts int) {
		check(func(r server.GetResponse) {
			if r.Status != s.String() {
				t.Fatalf("%s: expected %s status, got %s, restarts %d", description, s.String(), r.Status, r.Restarts)
			}
			if r.Restarts != restarts {
				t.Fatalf("%s: expected %d restarts, got %d", description, restarts, r.Restarts)
			}
		})
	}

	// let the command run
	time.Sleep(25 * time.Millisecond)
	// initDelay 0.1 seconds
	checkSR("before first healthcheck", server.Pending, 0)
	time.Sleep(100 * time.Millisecond)
	if !common.IsPortOpen(27000) {
		t.Errorf("port 27000 is closed")
	}
	checkSR("first healthcheck ok", server.Running, 0)

	err := unix.KillByPort(27000)
	if err != nil {
		t.Fatalf("failed to kill: %s", err)
	}
	time.Sleep(100 * time.Millisecond)
	checkSR("second healthcheck ok", server.Running, 0)
	time.Sleep(125 * time.Millisecond) // 25 millies for it to restart
	checkSR("should have restarted", server.Pending, 1)
}

func TestShutdown_DeleteDeployments(t *testing.T) {
	cmd := exec.Command("go", "run", "main.go", "start")
	cmd.Dir = ".."
	cmd.Stdout = os.Stdout
	if cmd.Start() != nil {
		t.Fatal("failed to start Yetis")
	}

	time.Sleep(500 * time.Millisecond)
	applyNC(t)
	time.Sleep(25 * time.Millisecond)
	if !common.IsPortOpen(27000) {
		t.Fatal("nc haven't started")
	}
	client.ShutdownServer()
	if common.IsPortOpen(27000) {
		t.Fatal("nc should've stopped")
	}
}

func applyNC(t *testing.T) {
	t.Helper()
	fullPath, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	configs, err := common.ReadConfigs(fullPath + "/nc/nc.yaml")
	if err != nil {
		t.Fatalf("failed to read config: %s", err)
	}
	res, err := fetch.Post[server.PostResponse]("", configs)
	if err != nil {
		t.Fatal("failed to apply configs", err, res)
	}
	if res.Success != 1 {
		t.Fatalf("failed to apply config: %s", res.Error)
	}
}
