package itests

import (
	"context"
	"errors"
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

// noticed ci fail
//
//	server_test.go:40: should have restarted: expected Pending status, got Terminating, restarts 0
//
// and
//
//	server_test.go:42: before first healthcheck: expected Pending status, got Running, restarts 0
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

	// initDelay 0.1 seconds
	checkSR("before first heartbeat", server.Pending, 0)
	time.Sleep(125 * time.Millisecond)
	checkSR("after first heartbeat", server.Running, 0)

	err := unix.KillByPort(27000, true)
	if err != nil {
		t.Fatalf("failed to kill: %s", err)
	}
	time.Sleep(95 * time.Millisecond)
	checkSR("after second heartbeat", server.Running, 0)

	time.Sleep(150 * time.Millisecond) // 50 millies for it to restart
	check(func(r server.GetResponse) {
		if r.Restarts != 1 {
			t.Fatal("expected a restart")
		}
	})
	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 20) {
		t.Fatal("deployment port should be open after restart")
	}
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

func TestServiceUpdatesWhenDeploymentRestartsOnLivenessFailure(t *testing.T) {
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
		res, err := fetch.Get[string]("http://localhost:27000/hello", fetch.Config{Timeout: time.Second})
		if err != nil {
			t.Error(err)
		}
		if res != "OK" {
			t.Errorf("wrong response %v", res)
		}
	}
	checkOK()

	err = unix.KillByPort(deps[0].LivenessPort, true)
	if err != nil {
		t.Fatal(err)
	}
	// for some insane reason the first call from within the test after killing the deployment can't connect to the tcp proxy.
	// here the real error should be "Connection reset by peer" not the timeout.
	_, err = fetch.Get[string]("http://localhost:27000/hello", fetch.Config{Timeout: 10 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("expected context deadline, got", err)
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

	newDeps, err := server.ListDeployment()
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
	checkOK()
}

func TestRestartRollingUpdate_ZeroDowntime(t *testing.T) {
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(5 * time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/main-rolling-update.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}
	client.GetServices()
	forTimeout(time.Second, func() bool {
		dep, err := client.GetDeployment("go")
		if err != nil {
			t.Fatal(err)
		}
		if dep.Status == server.Running.String() {
			return false
		}
		return true
	})
	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 50) {
		t.Fatal("service port should be open")
	}

	// checking zero downtime
	for i := 0; i < 5; i++ {
		go func() {
			res, err := fetch.Get[string]("http://localhost:27000/hello", fetch.Config{Timeout: time.Second})
			if err != nil {
				t.Error(err)
			}
			if res != "OK" {
				t.Errorf("wrong response %v", res)
			}
		}()
	}

	err := client.Restart("go")
	if err != nil {
		t.Fatal(err)
	}

	forTimeout(time.Second, func() bool {
		dep, err := client.GetDeployment("go-1")
		if err != nil {
			t.Fatal(err)
		}
		if dep.Status == server.Running.String() {
			return false
		}
		return true
	})
}

func TestServiceLivenessRestart(t *testing.T) {
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(5 * time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/main-with-service.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 20) {
		t.Fatal("service port should be open")
	}

	err := unix.KillByPort(27000, true)
	if err != nil {
		t.Fatal(err)
	}

	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 20) {
		t.Fatal("server should be restarted")
	}
}

func TestDeploymentRestartWithNewYetisPort(t *testing.T) {
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(5 * time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/main.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	dep, err := client.GetDeployment("go")
	if err != nil {
		t.Fatal(err)
	}

	if !common.IsPortOpenRetry(dep.Spec.YetisPort(), 50*time.Millisecond, 20) {
		t.Fatal("deployment port should be open", dep.Spec.YetisPort())
	}

	err = unix.KillByPort(dep.Spec.YetisPort(), true)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)

	dep, err = client.GetDeployment("go")
	if err != nil {
		t.Fatal(err)
	}

	if common.IsPortOpenRetry(dep.Spec.YetisPort(), 50*time.Millisecond, 20) {
		t.Fatal("server should be restarted")
	}
}

func pwd(t *testing.T) string {
	fullPath, err := os.Getwd()
	if err != nil {
		t.Fatalf("pwd: %s", err)
	}
	return fullPath
}

// return false to break the loop.
func forTimeout(timeout time.Duration, apply func() bool) {
	ch := time.After(timeout)
loop:
	for {
		select {
		case <-ch:
		default:
			if !apply() {
				break loop
			}
		}
	}
}
