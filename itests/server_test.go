package itests

import (
	"fmt"
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
	go server.Run()
	t.Cleanup(server.Stop)
	// let the server start
	time.Sleep(time.Millisecond)

	errs := client.Apply(pwd(t) + "/specs/nc.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	check := func(f func(server.GetResponse)) {
		dr, err := client.GetDeployment("hello")
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
