package itests

import (
	"github.com/glossd/fetch"
	_ "github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/server"
	"os"
	"testing"
	"time"
)

func TestRestart(t *testing.T) {
	os.Setenv("YETIS_SERVER_LOGDIR", "stdout")
	go server.Start()
	// let the server start
	time.Sleep(time.Millisecond)
	configs, err := common.ReadConfigs("./testrestart/test.yaml")
	if err != nil {
		t.Fatalf("failed to read config: %s", err)
	}
	res, err := fetch.Post[server.PostResponse]("", configs)
	if err != nil {
		t.Fatal("failed to apply configs", err, res)
	}
	if res.Success != 1 {
		t.Fatalf("failed to apply config")
	}
	defer server.KillByPort(27000)

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
	if !server.IsPortOpen(27000, time.Second) {
		t.Errorf("port 27000 is closed")
	}
	checkSR("first healthcheck ok", server.Running, 0)

	err = server.KillByPort(27000)
	if err != nil {
		t.Fatalf("failed to kill: %s", err)
	}
	time.Sleep(100 * time.Millisecond)
	checkSR("second healthcheck ok", server.Running, 0)
	time.Sleep(125 * time.Millisecond) // 25 millies for it to restart
	checkSR("should have restarted", server.Pending, 1)
}
