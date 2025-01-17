package main

import (
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"net/http"
	"testing"
	"time"
)

func TestProxyingHTTP(t *testing.T) {
	httpServerPort := 15465
	mux := &http.ServeMux{}
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`Hello World`))
	})
	go http.ListenAndServe(fmt.Sprintf(":%d", httpServerPort), mux)
	proxyPort := 24634
	go Start(proxyPort, httpServerPort, 45796)
	time.Sleep(5 * time.Millisecond)
	for i := 0; i < 5; i++ {
		res, err := fetch.Get[string](fmt.Sprintf("http://localhost:%d/hello", proxyPort))
		if err != nil {
			t.Fatal(err)
		}
		if res != `Hello World` {
			t.Errorf("wrong body, got %s", res)
		}
	}
}

func TestProxyingUpdatePort(t *testing.T) {
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
	proxyHttpPort := 47365

	go func() {
		Start(proxyPort, firstServerPort, proxyHttpPort)
	}()
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
	for i := 0; i < 3; i++ {
		go func() {
			for {
				checkOK()
			}
		}()
	}

	_, err := fetch.Post[fetch.Empty](fmt.Sprintf("http://localhost:%d/update", proxyHttpPort), secondServerPort)
	if err != nil {
		t.Fatal(err)
	}
	checkOK()
	firstServer.Shutdown(context.Background())
	time.Sleep(100 * time.Millisecond) // let the goroutines do the work
}
