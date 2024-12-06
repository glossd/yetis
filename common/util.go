package common

import (
	"fmt"
	"net"
	"time"
)

// IsPortOpen tries to establish a TCP connection to the specified address and port
func IsPortOpen(port int) bool {
	return IsPortOpenTimeout(port, time.Second)
}

func IsPortOpenTimeout(port int, timeout time.Duration) bool {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func IsPortOpenUntil(port int, period time.Duration, maxRestarts int) bool {
	if maxRestarts == 0 {
		return false
	}
	if IsPortOpen(port) {
		return true
	}
	time.Sleep(period)
	return IsPortOpenUntil(port, period, maxRestarts-1)
}
