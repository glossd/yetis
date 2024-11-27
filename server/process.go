package server

import (
	"context"
	"fmt"
	"github.com/glossd/yetis/common"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var logNamePattern = regexp.MustCompile("^[a-zA-Z]+-(\\d+).log$")

func launchProcess(c common.Config) (int, error) {
	if c.Spec.Logdir == "stdout" {
		return launchProcessWithOut(c, nil, false)
	} else {
		logName := c.Spec.Name + "-" + strconv.Itoa(getLogCounter(c.Spec.Name, c.Spec.Logdir)+1) + ".log"
		fullPath := c.Spec.Logdir + "/" + logName
		file, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0750)
		if err != nil {
			return 0, fmt.Errorf("failed to create log file for '%s': %s", c.Spec.Name, err)
		}
		return launchProcessWithOut(c, file, false)
	}
}

func launchProcessWithOut(c common.Config, w io.Writer, wait bool) (int, error) {
	var ev strings.Builder
	for i, envVar := range c.Spec.Env {
		if i > 0 {
			ev.WriteString(" ")
		}
		val := envVar.Value
		if strings.Contains(envVar.Value, "'") {
			// escaping single quotes.
			val = strings.ReplaceAll(val, "'", `'\''`)
		}
		ev.WriteString(fmt.Sprintf("%s='%s'", envVar.Name, val))
	}
	shCmd := append([]string{"sh", "-c", ev.String() + " " + c.Spec.Cmd})
	cmd := exec.Command(shCmd[0], shCmd[1:]...)
	if w != nil {
		cmd.Stdout = w
	}
	cmd.Dir = c.Spec.Workdir
	var err error
	if wait {
		err = cmd.Run()
	} else {
		err = cmd.Start()
	}
	if err != nil {
		err = fmt.Errorf("failed to start '%s' command: %s", c.Spec.Cmd, err)
		log.Println(err)
		return 0, err
	}
	pid := cmd.Process.Pid
	log.Printf("launched '%s' deployment, pid=%d\n", c.Spec.Name, pid)
	if pid == 0 {
		log.Printf("pid of %s is zero value", c.Spec.Name)
		return 0, fmt.Errorf("pid is zero")
	}
	return pid, nil
}

// Blocking. Once context expires, it sends SIGKILL.
func TerminateProcess(ctx context.Context, pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("couldn't find process %d: %s", pid, err)
	}

	// todo delete all children processes created by the command.
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to terminate %d process: %s", pid, err)
	}

	// Wait until the process terminates
	for {
		select {
		case <-ctx.Done():
			err = process.Signal(syscall.SIGKILL)
			if err != nil {
				log.Printf("failed to kill %d process: %s\n", pid, err)
			}
			return nil
		default:
			if !IsProcessAlive(process.Pid) {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func IsProcessAlive(pid int) bool {
	// 'ps -o pid= -p $PID' command works on MacOS and Linux
	res, err := exec.Command("ps", "-o", "pid=", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		if err.Error() != "exit status 1" {
			log.Printf("ps -o pid= -p failed: %s\n", err)
		}
		return false
	}
	if len(res) > 0 {
		if strings.HasSuffix(strings.TrimSpace(string(res)), "<defunct>") {
			return false
		}
		fmt.Printf("IsProcessAlive output: %s\n", res)
		return true
	}
	return false
}

func GetPidByPort(port int) (int, error) {
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
	output, err := cmd.Output()
	if err != nil {
		if err.Error() == "exit status 1" {
			return 0, fmt.Errorf("port %d is closed", port)
		}
		return 0, fmt.Errorf("searching for port %d: %s", port, err)
	}
	if len(output) == 0 {
		return 0, fmt.Errorf("port %d is closed", port)
	}

	pids := strings.Split(string(output), "\n")
	pid, err := strconv.Atoi(pids[0])
	if err != nil {
		panic("failed to convert string pid to int, pid=" + pids[0] + "," + err.Error())
	}
	return pid, nil
}

func KillByPort(port int) error {
	pid, err := GetPidByPort(port)
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = proc.Kill()
	if err != nil {
		return err
	}
	return nil
}

var isPortOpenMock *bool

// IsPortOpen tries to establish a TCP connection to the specified address and port
func IsPortOpen(port int, timeout time.Duration) bool {
	if isPortOpenMock != nil {
		return *isPortOpenMock
	}
	address := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func getLogCounter(name, logDir string) int {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		log.Printf("Couldn't read logdir: %s\n", err)
		return -1
	}
	var highest = -1
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), name+"-") && strings.HasSuffix(entry.Name(), ".log") {
			numStr := entry.Name()[(len(name) + 1):(len(entry.Name()) - 4)]
			num, err := strconv.Atoi(numStr)
			if err == nil {
				highest = max(num, highest)
			}
		}
	}
	return highest
}
