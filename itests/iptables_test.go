package itests

import (
	"errors"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"github.com/glossd/yetis/server"
	"os"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestProxyUpdatesWhenDeploymentRestartsOnLivenessFailure(t *testing.T) {
	skipIfNotIptables(t)
	go server.Run("")
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
	forTimeout(t, 2*time.Second, func() bool {
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
	skipIfNotIptables(t)
	go server.Run("")
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
	skipIfNotIptables(t)
	go server.Run("")
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

func TestRestartThroughApply_RollingUpdateStrategy(t *testing.T) {
	skipIfNotIptables(t)

	go server.Run("")
	t.Cleanup(server.Stop)
	time.Sleep(time.Millisecond)
	errs := client.Apply(pwd(t) + "/specs/app-port.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	firstDep, err := client.GetDeployment("go")
	assert(t, err, nil)

	checkDeploymentRunning(t, "go")

	errs = client.Apply(pwd(t) + "/specs/app-port.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	checkDeploymentRunning(t, "go-1")

	secondDep, err := client.GetDeployment("go-1")
	assert(t, err, nil)
	if firstDep.Spec.LivenessProbe.Port() == secondDep.Spec.LivenessProbe.Port() {
		t.Error("ports supposed to be different")
	}
}

func skipIfNotIptables(t *testing.T) {
	if os.Getenv("TEST_IPTABLES") == "" {
		t.SkipNow()
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
	forTimeout(t, 5*time.Second, func() bool {
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
	forTimeout(t, 5*time.Second, func() bool {
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
func forTimeout(t *testing.T, timeout time.Duration, apply func() bool) {
	ch := time.After(timeout)
loop:
	for {
		select {
		case <-ch:
			t.Helper()
			t.Fatal("forTimeout timeout")
		default:
			if !apply() {
				break loop
			}
		}
	}
}

func assert[T comparable](t *testing.T, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("got %v, wanted %v", got, want)
	}
}
