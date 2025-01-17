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
	if conn == nil {
		return false
	}
	conn.Close()
	return true
}

func IsPortOpenRetry(port int, period time.Duration, maxRestarts int) bool {
	if maxRestarts == 0 {
		return false
	}
	if IsPortOpenTimeout(port, period) {
		return true
	}
	return IsPortOpenRetry(port, period, maxRestarts-1)
}

// GetFreePort asks the kernel for a free open port that is ready to use.
// https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}
