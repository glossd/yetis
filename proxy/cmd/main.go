package main

import (
	"fmt"
	"github.com/glossd/yetis/proxy"
	"os"
	"strconv"
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
	toPortStr := os.Args[2]
	toPort, err := strconv.Atoi(toPortStr)
	if err != nil {
		panic(fmt.Sprintf("service target port must be a number, got %s", toPortStr))
	}

	httpPortStr := os.Args[3]
	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		panic(fmt.Sprintf("service http port must be a number, got %s", toPortStr))
	}

	proxy.Start(port, toPort, httpPort)
}
