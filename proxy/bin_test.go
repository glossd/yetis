package proxy

import (
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"net/http"
	"testing"
	"time"
)

func TestExec(t *testing.T) {
	toPort := 45678

	mux := &http.ServeMux{}
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	go http.ListenAndServe(fmt.Sprintf(":%d", toPort), mux)
	time.Sleep(5 * time.Millisecond)
	for i := 0; i < 10; i++ {
		fmt.Println("Attempt", i)
		run(t, toPort)
	}
}

func run(t *testing.T, toPort int) {
	port := 4567
	pid, err := Exec(port, toPort)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.TerminateProcessTimeout(pid, time.Second)
	if pid <= 0 {
		t.Fatalf("got pid: %d", pid)
	}
	if !common.IsPortOpenRetry(port, 50*time.Millisecond, 20) {
		t.Fatal("proxy's port is closed")
	}
	res, err := fetch.Get[string](fmt.Sprintf("http://localhost:%d/hello", port), fetch.Config{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if res != "OK" {
		t.Fatal("failed to proxy to http server")
	}
}
