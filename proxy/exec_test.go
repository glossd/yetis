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

func TestCreatePortForwarding(t *testing.T) {
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

	err := CreatePortForwarding(port, targetPort)
	if err != nil {
		t.Fatal(err)
	}

	defer DeletePortForwarding(port, targetPort)

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
}

func TestUpdatePortForwarding(t *testing.T) {
	port := 4567
	fakeDeploymentPort := 45678
	err := CreatePortForwarding(port, fakeDeploymentPort)
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

	err = UpdatePortForwarding(port, fakeDeploymentPort, secondPort)
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

func TestExtractLineNumber(t *testing.T) {
	output := `
Chain OUTPUT (policy ACCEPT)
num target     prot opt  source       destination
1   REDIRECT   tcp  --   anywhere     anywhere          tcp dpt:1024 redir ports 8080
`
	num, err := extractLine(output, 1024, 8080)
	if err != nil {
		t.Fatal(err)
	}
	if num != 1 {
		t.Fatal("num mismatch")
	}
}
