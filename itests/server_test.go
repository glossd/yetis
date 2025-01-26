package itests

import (
	"errors"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/client"
	_ "github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"github.com/glossd/yetis/server"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"syscall"
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
	unix.KillByPort(server.YetisServerPort, true)
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/simple-app-port.yaml")
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

	checkSR("before first heartbeat", server.Pending, 0)
	time.Sleep(1125 * time.Millisecond) // initDelay 1 second
	checkSR("after first heartbeat", server.Running, 0)

	err := unix.KillByPort(27000, true)
	if err != nil {
		t.Fatalf("failed to kill: %s", err)
	}
	time.Sleep(95 * time.Millisecond)
	checkSR("after second heartbeat", server.Running, 0)

	time.Sleep(200 * time.Millisecond)
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
	errs := client.Apply(pwd(t) + "/specs/simple-app-port.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 30) {
		t.Fatal("main haven't started")
	}
	client.ShutdownServer(100 * time.Millisecond)
	if common.IsPortOpenRetry(27000, 50*time.Millisecond, 10) {
		t.Fatal("main should've stopped")
	}
	if common.IsPortOpenRetry(server.YetisServerPort, 50*time.Millisecond, 10) {
		t.Fatal("server should've stopped")
	}
}

func TestProxyUpdatesWhenDeploymentRestartsOnLivenessFailure(t *testing.T) {
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(5 * time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/main-proxy.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	dep, err := client.GetDeployment("go")
	if err != nil {
		t.Fatal(err)
	}

	if !common.IsPortOpenRetry(dep.Spec.YetisPort(), 50*time.Millisecond, 20) {
		t.Fatal("deployment port closed", dep.Spec.YetisPort())
	}
	if !common.IsPortOpenRetry(dep.Spec.Proxy.Port, 50*time.Millisecond, 20) {
		t.Fatal("port forwarding closed", dep.Spec.Proxy.Port)
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

	err = unix.KillByPort(dep.Spec.YetisPort(), true)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fetch.Get[string]("http://localhost:27000/hello", fetch.Config{Timeout: 10 * time.Millisecond})
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Error("expected  connection refused, got", err)
	}
	forTimeout(time.Second, func() bool {
		dep, err := client.GetDeployment("go")
		if err != nil {
			t.Fatal(err)
		}
		if dep.Restarts == 1 {
			return false
		}
		return true
	})

	newDeps, err := server.ListDeployment()
	if err != nil {
		t.Fatal(err)
	}
	if len(newDeps) != 1 {
		t.Fatalf("expected one go deployment, got=%v", newDeps)
	}
	if dep.Spec.YetisPort() == newDeps[0].LivenessPort {
		t.Fatal("same deployment port, no restart")
	}
	if !common.IsPortOpenRetry(newDeps[0].LivenessPort, 50*time.Millisecond, 20) {
		t.Fatal("new deployment port closed", newDeps[0].LivenessPort)
	}
	checkOK()
}

func TestRestart_RollingUpdate_ZeroDowntime(t *testing.T) {
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(5 * time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/main-rolling-update.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}
	checkDeploymentRunning(t, "go")
	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 50) {
		t.Fatal("service port should be open")
	}
	firstDep, err := client.GetDeployment("go")
	if err != nil {
		t.Fatal(err)
	}

	var stop atomic.Bool

	// checking zero downtime
	for i := 0; i < 3; i++ {
		go func() {
			for {
				// the request might take more than a second to complete.
				res, err := fetch.Get[string]("http://localhost:27000/hello", fetch.Config{Timeout: 3 * time.Second})
				if stop.Load() {
					return
				}
				if err != nil {
					t.Error("Worker "+strconv.Itoa(i), time.Now(), err)
					continue
				}
				if res != "OK" {
					t.Errorf("wrong response %v", res)
				}
			}
		}()
	}

	err = client.Restart("go")
	if err != nil {
		t.Fatal(err)
	}

	checkDeploymentRunning(t, "go-1")
	secondDep, err := client.GetDeployment("go-1")
	if err != nil {
		t.Fatal(err)
	}
	if firstDep.Spec.YetisPort() == secondDep.Spec.YetisPort() {
		t.Fatal("restarted on the same port")
	}
	time.Sleep(3 * time.Second) // let http requests finish or timeout
	stop.Store(true)
}

func TestDeploymentRestartWithNewYetisPort(t *testing.T) {
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(5 * time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/app-port.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	firstDep, err := client.GetDeployment("go")
	if err != nil {
		t.Fatal(err)
	}

	checkDeploymentRunning(t, "go")

	err = unix.KillByPort(firstDep.Spec.YetisPort(), true)
	if err != nil {
		t.Fatal(err)
	}
	checkDeploymentRestarted(t, "go")

	secondDep, err := client.GetDeployment("go") // liveness does not upgrade the name
	if err != nil {
		t.Fatal(err)
	}

	if !common.IsPortOpenRetry(secondDep.Spec.YetisPort(), 50*time.Millisecond, 20) {
		t.Fatal("new deployment port should be open")
	}

	if secondDep.Spec.YetisPort() == firstDep.Spec.YetisPort() {
		t.Error("new rolled dep has the same port as the old one", secondDep.Spec.YetisPort())
	}
	if secondDep.Spec.GetEnv("APP_PORT") != "$YETIS_PORT" || firstDep.Spec.GetEnv("APP_PORT") != "$YETIS_PORT" {
		t.Error("env values with YETIS_PORT shouldn't change")
	}
}

func pwd(t *testing.T) string {
	fullPath, err := os.Getwd()
	if err != nil {
		t.Fatalf("pwd: %s", err)
	}
	return fullPath
}

func checkDeploymentRunning(t *testing.T, name string) {
	forTimeout(5*time.Second, func() bool {
		dep, err := client.GetDeployment(name)
		if err != nil {
			t.Fatal(err)
		}
		if dep.Status == server.Running.String() {
			return false
		}
		return true
	})
}

func checkDeploymentRestarted(t *testing.T, name string) {
	forTimeout(5*time.Second, func() bool {
		dep, err := client.GetDeployment(name)
		if err != nil {
			t.Fatal(err)
		}
		if dep.Restarts > 0 {
			return false
		}
		return true
	})
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
