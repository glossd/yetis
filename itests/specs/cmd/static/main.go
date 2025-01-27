package main

import (
	"io"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", ":27000")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		io.WriteString(conn, "OK")

		conn.Close()
	}
}
