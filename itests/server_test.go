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

func TestLivenessRestart(t *testing.T) {
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/nc.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	check := func(f func(server.GetResponse)) {
		dr, err := fetch.Get[server.GetResponse]("/deployments/hello")
		if err != nil {
			t.Fatal(err)
		}
		f(dr)
	}

	checkSR := func(description string, s server.ProcessStatus, restarts int) {
		check(func(r server.GetResponse) {
			if r.Status != s.String() {
				t.Fatalf("%s: expected %s status, got %s, restarts %d", description, s.String(), r.Status, r.Restarts)
			}
			if r.Restarts != restarts {
				t.Fatalf("%s: expected %d restarts, got %d", description, restarts, r.Restarts)
			}
		})
	}

	if !common.IsPortOpenRetry(27000, 20*time.Millisecond, 20) {
		t.Fatalf("port 27000 is closed")
	}
	// initDelay 0.1 seconds
	checkSR("before first healthcheck", server.Pending, 0)
	time.Sleep(100 * time.Millisecond)
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
	cmd := exec.Command("go", "run", "main.go", "run")
	cmd.Dir = ".."
	cmd.Stdout = os.Stdout
	if cmd.Start() != nil {
		t.Fatal("failed to start Yetis")
	}

	if !common.IsPortOpenRetry(server.YetisServerPort, 50*time.Millisecond, 20) {
		t.Fatal("yetis server hasn't started")
	}
	errs := client.Apply(pwd(t) + "/specs/nc.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 30) {
		t.Fatal("nc haven't started")
	}
	client.ShutdownServer(100 * time.Millisecond)
	if common.IsPortOpenRetry(27000, 50*time.Millisecond, 10) {
		t.Fatal("nc should've stopped")
	}
	if common.IsPortOpenRetry(server.YetisServerPort, 50*time.Millisecond, 10) {
		t.Fatal("server should've stopped")
	}
}

func TestServiceUpdatesWhenDeploymentRestartsOnNewPort(t *testing.T) {
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(5 * time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/main-with-service.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	deps, err := fetch.Get[[]server.DeploymentView]("/deployments")
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 {
		t.Fatalf("got deployments: %v", deps)
	}
	sers, err := fetch.Get[[]server.ServiceView]("/services")
	if err != nil {
		t.Fatal(err)
	}
	if len(sers) != 1 {
		t.Fatalf("got services: %v", sers)
	}

	if !common.IsPortOpenRetry(sers[0].Port, 50*time.Millisecond, 20) {
		t.Fatal("service port closed", sers[0].Port)
	}
	if !common.IsPortOpenRetry(sers[0].DeploymentPort, 50*time.Millisecond, 20) {
		t.Fatal("deployment port closed", sers[0].DeploymentPort)
	}

	checkOK := func() {
		t.Helper()
		res, err := fetch.Get[string]("http://localhost:27000/hello", fetch.Config{Timeout: 100 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		if res != "OK" {
			t.Fatalf("wrong response %v", res)
		}
	}
	checkOK()

	err = unix.KillByPort(deps[0].LivenessPort)
	if err != nil {
		t.Fatal(err)
	}
	for {
		deps, err := fetch.Get[[]server.DeploymentView]("/deployments")
		if err != nil {
			t.Fatal(err)
		}
		if deps[0].Restarts == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	newDeps, err := server.ListDeployment(fetch.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if len(newDeps) != 1 {
		t.Fatalf("expected one go deployment, got=%v", newDeps)
	}
	if deps[0].LivenessPort == newDeps[0].LivenessPort {
		t.Fatal("same deployment port, no restart")
	}
	if !common.IsPortOpenRetry(newDeps[0].LivenessPort, 50*time.Millisecond, 20) {
		t.Fatal("new deployment port closed", newDeps[0].LivenessPort)
	}
	if !common.IsPortOpenRetry(sers[0].Port, 50*time.Millisecond, 20) {
		t.Fatal("service port closed", sers[0].Port)
	}
	checkOK()
}

func pwd(t *testing.T) string {
	fullPath, err := os.Getwd()
	if err != nil {
		t.Fatalf("pwd: %s", err)
	}
	return fullPath
}
