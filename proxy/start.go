package proxy

import (
	"fmt"
	"github.com/glossd/fetch"
	"io"
	"log"
	"net"
	"net/http"
	"sync/atomic"
)

var proxyToPort atomic.Int32

func Start(listenPort, toPort, httpPort int) {
	proxyToPort.Store(int32(toPort))
	lis, err := net.ListenTCP("tcp", &net.TCPAddr{Port: listenPort})
	if err != nil {
		panic("Listen: " + err.Error())
	}
	defer lis.Close()

	go func() {
		mux := &http.ServeMux{}
		mux.HandleFunc("/update", fetch.ToHandlerFunc(func(in int32) (*fetch.Empty, error) {
			proxyToPort.Store(in)
			return nil, nil
		}))
		serveErr := http.ListenAndServe(fmt.Sprintf(":%d", httpPort), mux)
		if serveErr != nil && serveErr != http.ErrServerClosed {
			log.Fatalf("Proxy http server failed to start: %s\n", err)
		}
	}()

	for {
		c, err := lis.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			return
		}
		go proxyTo(c, int(proxyToPort.Load()))
	}
}

func proxyTo(inCon net.Conn, port int) {
	dialCon, err := net.Dial("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Println("Dial error:", err)
		return
	}
	defer dialCon.Close()

	go func() {
		_, err := io.Copy(dialCon, inCon)
		if err != nil {
			log.Printf("Error sending to service: %v", err)
		}
	}()

	_, err = io.Copy(inCon, dialCon)
	if err != nil {
		log.Printf("Error sending to client: %v", err)
	}
}
