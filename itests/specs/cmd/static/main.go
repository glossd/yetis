package main

import (
	"fmt"
	"io"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", ":27000")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	fmt.Println("static main: Starting to listen to tcp connections on 27000")
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		io.WriteString(conn, "OK")

		conn.Close()
	}
}
