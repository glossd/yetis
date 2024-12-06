package unix

import (
	"bytes"
	"context"
	"github.com/glossd/yetis/common"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestIsProcessAlive(t *testing.T) {
	cmd := exec.Command("sleep", "0.01")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("error launching process: %s", err)
	}
	pid := cmd.Process.Pid
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
	cmd := exec.Command("sleep", "10")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("error launching process: %s", err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	err = TerminateProcess(ctx, cmd.Process.Pid)
	if err != nil {
		t.Fatalf("failed to terminated the process: %s", err)
	}
}

func TestIsPortOpen(t *testing.T) {
	s := http.Server{Addr: ":44534"}
	go s.ListenAndServe()
	defer s.Shutdown(context.Background())
	time.Sleep(time.Millisecond)
	if !common.IsPortOpen(44534) {
		t.Errorf("port shouldn't be closed")
	}

	if common.IsPortOpen(34567) {
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

func TestCatStream(t *testing.T) {
	assert(t, os.Truncate("./cat.txt", 0), nil)
	buf := bytes.NewBuffer([]byte{})
	go func() {
		err := printFileTo("./cat.txt", buf, true)
		assert(t, err, nil)
	}()
	f, err := os.OpenFile("./cat.txt", os.O_WRONLY, os.ModeAppend)
	assert(t, err, nil)
	_, err = f.WriteString("Hello\n")
	assert(t, err, nil)
	time.Sleep(5 * time.Millisecond)
	assert(t, buf.String(), "Hello\n")
	_, err = f.WriteString("World\n")
	assert(t, err, nil)
	time.Sleep(15 * time.Millisecond)
	assert(t, buf.String(), "Hello\nWorld\n")
}

func assert[T comparable](t *testing.T, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("got %v, wanted %v", got, want)
	}
}

func TestExecutableExists(t *testing.T) {
	assert(t, ExecutableExists("cat"), true)
	assert(t, ExecutableExists("dskdywkcnoiuiuhjvncueiho"), false)
}
