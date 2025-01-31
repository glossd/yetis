package unix

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func TerminateProcessTimeout(pid int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return TerminateProcess(ctx, pid)
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
				log.Printf("context deadline exceeded but failed to kill %d process: %s\n", pid, err)
				return err
			}
			return context.DeadlineExceeded
		default:
			if !IsProcessAlive(process.Pid) {
				return nil
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TerminateGroupProcess(ctx context.Context, parentPid int) error {
	pgid, err := syscall.Getpgid(parentPid)
	if err != nil {
		return err
	}
	err = syscall.Kill(-pgid, syscall.SIGTERM)
	if err != nil {
		return err
	}

	// Wait until the process terminates, but think of the children! todo
	for {
		select {
		case <-ctx.Done():
			err = syscall.Kill(-pgid, syscall.SIGKILL)
			if err != nil {
				log.Printf("context deadline exceeded: failed to kill %d process group: %s\n", pgid, err)
				return err
			}
			return context.DeadlineExceeded
		default:
			if !IsProcessAlive(parentPid) {
				return nil
			}
			time.Sleep(5 * time.Millisecond)
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

func KillByPort(port int, wait bool) error {
	pid, err := GetPidByPort(port)
	if err != nil {
		return err
	}
	if !wait {
		return Kill(pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = proc.Kill()
	if err != nil {
		return err
	}
	proc.Wait()

	return nil
}

func Kill(pid int) error {
	return exec.Command("kill", "-9", strconv.Itoa(pid)).Start()
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
	_, err := exec.LookPath(executable)
	return err == nil
}

func DirContainsFile(dir, fileName string) bool {
	// Construct the full path to the file
	filePath := filepath.Join(dir, fileName)

	// Check if the file exists
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func IsExecutable(filepath string) bool {
	info, err := os.Stat(filepath)
	if err != nil {
		return false
	}

	if info.IsDir() {
		return false
	}

	return info.Mode()&0111 != 0
}
