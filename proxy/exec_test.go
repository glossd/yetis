package proxy

import (
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"net/http"
	"os"
	"os/signal"
	"testing"
	"time"
)

func TestExec(t *testing.T) {
	port := 4567
	unix.KillByPort(port, true)

	targetPort := 45678

	mux := &http.ServeMux{}
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	go http.ListenAndServe(fmt.Sprintf(":%d", targetPort), mux)
	if !common.IsPortOpenRetry(targetPort, 50*time.Millisecond, 20) {
		t.Fatal("target port should be open")
	}
	for i := 0; i < 10; i++ {
		fmt.Println("Attempt", i)
		run(t, port, targetPort)
	}
}

func run(t *testing.T, port, targetPort int) {
	pid, httpPort, err := Exec(port, targetPort, "/tmp/exec.log")
	if err != nil {
		t.Fatal(err)
	}
	defer unix.TerminateProcessTimeout(pid, time.Second)
	if pid <= 0 {
		t.Fatalf("got pid: %d", pid)
	}
	if !common.IsPortOpenRetry(port, 50*time.Millisecond, 30) {
		t.Fatal("proxy's port is closed")
	}
	res, err := fetch.Get[string](fmt.Sprintf("http://localhost:%d/hello", port), fetch.Config{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if res != "OK" {
		t.Fatal("failed to proxy to http server")
	}
	if !common.IsPortOpenRetry(httpPort, 50*time.Millisecond, 20) {
		t.Fatal("http port is closed")
	}
}

func TestExecChangePort(t *testing.T) {
	port := 4567
	fakeDeploymentPort := 45678
	_, httpPort, err := Exec(port, fakeDeploymentPort, "/tmp/exec-test.log")
	if err != nil {
		t.Fatal(err)
	}
	if !common.IsPortOpenRetry(port, 50*time.Millisecond, 20) {
		t.Fatal("service hasn't started")
	}

	secondPort := 45679
	go func() {
		mux := &http.ServeMux{}
		mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("OK"))
		})
		(&http.Server{Addr: fmt.Sprintf(":%d", secondPort), Handler: mux}).ListenAndServe()
	}()
	if !common.IsPortOpenRetry(secondPort, 50*time.Millisecond, 20) {
		t.Fatal("second deployment port should be open")
	}

	_, err = fetch.Post[fetch.Empty](fmt.Sprintf("http://localhost:%d/update", httpPort), secondPort)
	if err != nil {
		t.Fatal(err)
	}

	res, err := fetch.Get[string](fmt.Sprintf("http://localhost:%d/hello", port), fetch.Config{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if res != "OK" {
		t.Fatal("failed to proxy to http server")
	}
}

func startServer(mux *http.ServeMux, port int, stop chan os.Signal) {
	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			// handle err
		}
	}()

	signal.Notify(stop, os.Interrupt)

	// Waiting for SIGINT (kill -2)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		// handle err
	}
}
