package unix

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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
		return true
	}
	return false
}

func GetPidByPort(port int) (int, error) {
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-sTCP:LISTEN", "-t")
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
	var filtered []string
	for _, p := range pids {
		if strings.TrimSpace(p) != "" {
			filtered = append(filtered, p)
		}
	}
	pids = filtered
	if len(pids) > 1 {
		fmt.Println("Warning: GetPidByPort has more than one pid, count=", len(pids))
	}
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

func Cat(filePath string, stream bool) error {
	return printFileTo(filePath, os.Stdout, stream)
}
func printFileTo(filePath string, w io.Writer, stream bool) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %s", err)
	}
	buf := make([]byte, 1024)
	for {
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			if stream {
				time.Sleep(10 * time.Millisecond)
				continue
			} else {
				return nil
			}
		}
		_, err = w.Write(buf[:n])
		if err != nil {
			return err
		}
	}
}

func ExecutableExists(executable string) bool {
	out, err := exec.Command("command", "-v", executable).Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}
