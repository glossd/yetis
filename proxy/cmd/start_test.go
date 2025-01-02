package main

//import (
//	"fmt"
//	"github.com/glossd/fetch"
//	"net/http"
//	"testing"
//	"time"
//)
//
//func TestProxyingHTTP(t *testing.T) {
//	httpServerPort := 15465
//	mux := &http.ServeMux{}
//	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
//		w.WriteHeader(200)
//		w.Write([]byte(`Hello World`))
//	})
//	go http.ListenAndServe(fmt.Sprintf(":%d", httpServerPort), mux)
//	proxyPort := 24634
//	go Start(proxyPort, httpServerPort, 45796)
//	time.Sleep(5 * time.Millisecond)
//	for i := 0; i < 5; i++ {
//		res, err := fetch.Get[string](fmt.Sprintf("http://localhost:%d/hello", proxyPort))
//		if err != nil {
//			t.Fatal(err)
//		}
//		if res != `Hello World` {
//			t.Errorf("wrong body, got %s", res)
//		}
//	}
//}
//
//func TestProxyingUpdatePort(t *testing.T) {
//	httpServerPort := 15465
//	mux := &http.ServeMux{}
//	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
//		w.WriteHeader(200)
//		w.Write([]byte(`Hello World`))
//	})
//	go http.ListenAndServe(fmt.Sprintf(":%d", httpServerPort), mux)
//	proxyPort := 24636
//	proxyHttpPort := 47365
//	fakeServerPort := 46384
//	go func() {
//		Start(proxyPort, fakeServerPort, proxyHttpPort)
//	}()
//	time.Sleep(5 * time.Millisecond)
//
//	_, err := fetch.Post[fetch.Empty](fmt.Sprintf("http://localhost:%d/update", proxyHttpPort), httpServerPort)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	res, err := fetch.Get[string](fmt.Sprintf("http://localhost:%d/hello", proxyPort))
//	if err != nil {
//		t.Fatal(err)
//	}
//	if res != `Hello World` {
//		t.Errorf("wrong body, got %s", res)
//	}
//}
