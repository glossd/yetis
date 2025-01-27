package proxy

import (
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestPortForwarding(t *testing.T) {
	skipIfNotIptables(t)
	port := 4567
	targetPort := 45678
	unix.KillByPort(port, true)
	unix.KillByPort(targetPort, true)

	mux := &http.ServeMux{}
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", targetPort), mux)
		if err != nil {
			t.Error(err)
		}
	}()
	if !common.IsPortOpenRetry(targetPort, 50*time.Millisecond, 30) {
		t.Fatal("target port should be open")
	}

	err := CreatePortForwarding(port, targetPort)
	if err != nil {
		t.Fatal(err)
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

	err = DeletePortForwarding(port, targetPort)
	if err != nil {
		t.Fatal(err)
	}
	if common.IsPortOpenRetry(port, 50*time.Millisecond, 30) {
		t.Fatal("proxy's port should be closed")
	}
}

func TestUpdatePortForwarding(t *testing.T) {
	skipIfNotIptables(t)
	firstServerPort := 25465
	secondServerPort := 15465
	mux := &http.ServeMux{}
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`Hello World`))
	})

	firstServer := http.Server{Addr: fmt.Sprintf(":%d", firstServerPort), Handler: mux}
	go firstServer.ListenAndServe()
	go http.ListenAndServe(fmt.Sprintf(":%d", secondServerPort), mux)
	proxyPort := 24636

	err := CreatePortForwarding(proxyPort, firstServerPort)
	if err != nil {
		t.Fatal(err)
	}

	if !common.IsPortOpenRetry(proxyPort, 10*time.Millisecond, 10) {
		t.Fatal("proxy port is closed")
	}
	checkOK := func() {
		res, err := fetch.Get[string](fmt.Sprintf("http://localhost:%d/hello", proxyPort))
		if err != nil {
			t.Error(err)
		}
		if res != `Hello World` {
			t.Errorf("wrong body, got %s", res)
		}
	}
	checkOK()
	for i := 0; i < 10; i++ {
		go func() {
			for {
				checkOK()
			}
		}()
	}

	err = UpdatePortForwarding(proxyPort, firstServerPort, secondServerPort)
	if err != nil {
		t.Fatal(err)
	}
	checkOK()
	firstServer.Shutdown(context.Background())
	time.Sleep(100 * time.Millisecond) // let the goroutines do the work
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

func skipIfNotIptables(t *testing.T) {
	if os.Getenv("TEST_IPTABLES") == "" {
		t.SkipNow()
	}
}
