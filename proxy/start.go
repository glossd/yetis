package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
)

func Start(listenPort, toPort int) {
	lis, err := net.ListenTCP("tcp", &net.TCPAddr{Port: listenPort})
	if err != nil {
		panic("Listen: " + err.Error())
	}
	defer lis.Close()

	for {
		c, err := lis.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			return
		}
		go proxyTo(c, toPort)
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
