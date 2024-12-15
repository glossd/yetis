package proxy

import (
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"testing"
	"time"
)

func TestExec(t *testing.T) {
	port := 4567
	pid, err := Exec(port, 45679)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.KillByPort(port)
	if pid <= 0 {
		t.Fatalf("got pid: %d", pid)
	}
	if !common.IsPortOpenRetry(port, 50*time.Millisecond, 20) {
		t.Fatal("proxy's port is closed")
	}
}
