package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
)

func main() {
	if len(os.Args) != 4 {
		panic("service must have listen, target and http ports")
	}
	portStr := os.Args[1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(fmt.Sprintf("service listen port must be a number, got %s", portStr))
	}
	targetPortStr := os.Args[2]
	targetPort, err := strconv.Atoi(targetPortStr)
	if err != nil {
		panic(fmt.Sprintf("service target port must be a number, got %s", targetPortStr))
	}

	httpPortStr := os.Args[3]
	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		panic(fmt.Sprintf("service http port must be a number, got %s", targetPortStr))
	}

	Start(port, targetPort, httpPort)
}

var targetPortVar atomic.Int32

func Start(listenPort, targetPort, httpPort int) {
	targetPortVar.Store(int32(targetPort))
	lis, err := net.ListenTCP("tcp", &net.TCPAddr{Port: listenPort})
	if err != nil {
		panic("Listen: " + err.Error())
	}
	defer lis.Close()

	go func() {
		// todo replace http with tcp to reduce the binary size
		mux := &http.ServeMux{}
		mux.HandleFunc("POST /update", func(w http.ResponseWriter, r *http.Request) {
			all, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(400)
				w.Write([]byte("couldn't read: " + err.Error()))
			}
			defer r.Body.Close()
			var newPort int32
			err = json.Unmarshal(all, &newPort)
			if err != nil {
				w.WriteHeader(400)
				w.Write([]byte("expected a number in body: " + err.Error()))
			}
			fmt.Printf("Switching to new port: %d\n", newPort)
			targetPortVar.Store(newPort)

		})

		serveErr := http.ListenAndServe(fmt.Sprintf(":%d", httpPort), mux)
		if serveErr != nil && serveErr != http.ErrServerClosed {
			panic(fmt.Sprintf("Proxy http server failed to start: %s\n", err))
		}
	}()

	fmt.Println("Starting to listen on port", listenPort, "proxying to", targetPort)
	for {
		c, err := lis.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			return
		}
		fmt.Println("Accept", c.LocalAddr(), c.RemoteAddr())
		go proxyTo(c, int(targetPortVar.Load()))
	}
}

func proxyTo(inCon net.Conn, port int) {
	defer inCon.Close()
	dialCon, err := net.Dial("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Println("Dial error:", err)
		return
	}
	defer dialCon.Close()

	done := make(chan bool)
	go func() {
		_, err := io.Copy(dialCon, inCon)
		if err != nil {
			fmt.Printf("Error sending to service: %v\n", err)
		}
		done <- true
	}()

	_, err = io.Copy(inCon, dialCon)
	if err != nil {
		fmt.Printf("Error sending to deployment: %v\n", err)
	}
	<-done
}
