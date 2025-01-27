package common

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// IsPortOpen tries to establish a TCP connection to the specified address and port
func IsPortOpen(port int) bool {
	return DialPort(port, time.Second) == nil
}

func DialPort(port int, timeout time.Duration) error {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", address, timeout)

	if err != nil {
		return err
	}
	if conn != nil {
		conn.Close()
	}
	return nil
}

func IsPortOpenRetry(port int, period time.Duration, maxRestarts int) bool {
	if maxRestarts == 0 {
		return false
	}
	err := DialPort(port, period)
	if err == nil {
		return true
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		time.Sleep(period)
	}
	return IsPortOpenRetry(port, period, maxRestarts-1)
}

func IsPortCloseRetry(port int, period time.Duration, maxRestarts int) bool {
	if maxRestarts == 0 {
		return false
	}
	err := DialPort(port, period)
	if err != nil {
		return true
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		time.Sleep(period)
	}
	return IsPortCloseRetry(port, period, maxRestarts-1)
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

func MustGetFreePort() int {
	a, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic("ResolveTCPAddr: " + err.Error())
	}
	var l *net.TCPListener
	l, err = net.ListenTCP("tcp", a)
	if err != nil {
		panic("ListenTCP: " + err.Error())
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}
