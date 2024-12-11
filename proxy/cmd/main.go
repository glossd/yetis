package main

import (
	"fmt"
	"github.com/glossd/yetis/proxy"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 3 {
		panic("proxy must have ingress and egress port")
	}
	portStr := os.Args[1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(fmt.Sprintf("proxy listening port must be a number, got %s", portStr))
	}
	toPortStr := os.Args[2]
	toPort, err := strconv.Atoi(toPortStr)
	if err != nil {
		panic(fmt.Sprintf("proxy to port must be a number, got %s", toPortStr))
	}

	proxy.Start(port, toPort)
}
