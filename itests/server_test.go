package itests

import (
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/client"
	_ "github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"github.com/glossd/yetis/server"
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
	unix.KillByPort(server.YetisServerPort, true)
	go server.Run("")
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/nc.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	check := func(f func(server.DeploymentFullInfo)) {
		dr, err := client.GetDeployment("hello")
		if err != nil {
			t.Fatal(err)
		}
		f(dr)
	}

	checkSR := func(description string, s server.ProcessStatus, restarts int) {
		check(func(r server.DeploymentFullInfo) {
			if r.Status != s.String() {
				t.Fatalf("%s: expected %s status, got %s, restarts %d", description, s.String(), r.Status, r.Restarts)
			}
			if r.Restarts != restarts {
				t.Fatalf("%s: expected %d restarts, got %d", description, restarts, r.Restarts)
			}
		})
	}

	checkSR("before first heartbeat", server.Pending, 0)
	forTimeout(t, 3*time.Second, func() bool {
		d, err := client.GetDeployment("hello")
		assert(t, err, nil)
		if d.Status == server.Running.String() {
			return false
		}
		return true
	})

	fmt.Println("Killing hello")
	err := unix.KillByPort(27000, false)
	if err != nil {
		t.Fatalf("failed to kill: %s", err)
	}
	checkSR("needs two heartbeat failures to restart", server.Running, 0)

	forTimeout(t, 2*time.Second, func() bool {
		d, err := client.GetDeployment("hello")
		assert(t, err, nil)
		if d.Restarts == 1 {
			return false
		}
		return true
	})
	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 20) {
		t.Fatal("deployment port should be open after restart")
	}
}

func TestRestartThroughApply_RecreateStrategy(t *testing.T) {
	go server.Run("")
	t.Cleanup(server.Stop)
	time.Sleep(time.Millisecond)
	errs := client.Apply(pwd(t) + "/specs/nc.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	oldD, err := client.GetDeployment("hello")
	assert(t, err, nil)

	checkDeploymentRunning(t, "hello")

	res, err := fetch.Post[server.CRDeploymentResponse]("/deployments", common.DeploymentSpec{
		Name:          "hello",
		Cmd:           "nc -lk 27000",
		Logdir:        "stdout",
		LivenessProbe: common.Probe{TcpSocket: common.TcpSocket{Port: 27000}},
		Env:           []common.EnvVar{{Name: "NEW_ENV", Value: "Hello World"}},
	}.WithDefaults())
	assert(t, err, nil)
	assert(t, res.Existed, true)
	d, err := client.GetDeployment("hello")
	assert(t, err, nil)
	if oldD.Pid == d.Pid {
		t.Errorf("expected to restart the deployment: %+v", d)
	}
	assert(t, oldD.Spec.GetEnv("NEW_ENV"), "")
	if d.Spec.GetEnv("NEW_ENV") != "Hello World" {
		t.Errorf("expected new env to be set from the restart: %+v", d)
	}
}

func TestShutdown_DeleteDeployments(t *testing.T) {
	cmd := exec.Command("go", "run", "main.go", "run")
	cmd.Dir = ".."
	if cmd.Start() != nil {
		t.Fatal("failed to start Yetis")
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
	})

	if !common.IsPortOpenRetry(server.YetisServerPort, 50*time.Millisecond, 30) {
		t.Fatal("yetis server hasn't started")
	}
	errs := client.Apply(pwd(t) + "/specs/nc.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	if !common.IsPortOpenRetry(27000, 50*time.Millisecond, 30) {
		t.Fatal("main haven't started")
	}
	client.ShutdownServer(time.Second)
	if !common.IsPortCloseRetry(27000, 50*time.Millisecond, 10) {
		t.Fatal("main should've stopped")
	}
	if !common.IsPortCloseRetry(server.YetisServerPort, 50*time.Millisecond, 10) {
		t.Fatal("server should've stopped")
	}
}

func TestDeleteAllDeploymentChildProcesses(t *testing.T) {
	//unix.KillByPort(27000, true)
	//unix.KillByPort(27001, true)
	go server.Run("")
	t.Cleanup(server.Stop)
	time.Sleep(time.Millisecond)
	errs := client.Apply(pwd(t) + "/specs/subproc.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	checkDeploymentRunning(t, "subproc")
	// check subprocess is running
	if !common.IsPortOpenRetry(27001, 20*time.Millisecond, 5) {
		t.Fatal("subprocess isn't running")
	}
	err := client.DeleteDeployment("subproc")
	assert(t, err, nil)

	if !common.IsPortCloseRetry(27000, 20*time.Millisecond, 5) {
		t.Errorf("main process should be dead")
	}
	if !common.IsPortCloseRetry(27001, 20*time.Millisecond, 5) {
		t.Errorf("subprocess should be dead")
	}
}
