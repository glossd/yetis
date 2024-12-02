package common

import (
	"fmt"
	"net"
	"time"
)

// IsPortOpen tries to establish a TCP connection to the specified address and port
func IsPortOpen(port int, timeout time.Duration) bool {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
