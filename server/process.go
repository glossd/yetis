package server

import (
	"fmt"
	"github.com/glossd/yetis/common"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var logNamePattern = regexp.MustCompile("^[a-zA-Z]+-(\\d+).log$")

func launchProcess(c common.DeploymentSpec) (pid int, logPath string, err error) {
	if c.Logdir == "stdout" {
		pid, err = launchProcessWithOut(c, nil, false)
		return pid, "stdout", err
	} else {
		logName := c.Name + "-" + strconv.Itoa(getLogCounter(c.Name, c.Logdir)+1) + ".log"
		fullPath := c.Logdir + "/" + logName
		file, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0750)
		if err != nil {
			return 0, "", fmt.Errorf("failed to create log file for '%s': %s", c.Name, err)
		}
		pid, err = launchProcessWithOut(c, file, false)
		return pid, fullPath, err
	}
}

func launchProcessWithOut(c common.DeploymentSpec, w io.Writer, wait bool) (int, error) {
	if c.PreCmd != "" {
		cmd := exec.Command("sh", "-c", c.PreCmd)
		cmd.Dir = c.Workdir
		err := cmd.Run()
		if err != nil {
			return 0, fmt.Errorf("running precmd error: %s", err)
		}
	}
	var ev strings.Builder
	for i, envVar := range c.Env {
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
	shCmd := append([]string{"sh", "-c", ev.String() + " " + c.Cmd})
	cmd := exec.Command(shCmd[0], shCmd[1:]...)
	if w != nil {
		cmd.Stdout = w
	}
	cmd.Dir = c.Workdir
	var err error
	if wait {
		err = cmd.Run()
	} else {
		err = cmd.Start()
	}
	if err != nil {
		err = fmt.Errorf("failed to start '%s' command: %s", c.Cmd, err)
		log.Println(err)
		return 0, err
	}
	pid := cmd.Process.Pid

	if isYetisPortUsed(c) {
		log.Printf("launched '%s' deployment with port=%d, pid=%d\n", c.Name, c.LivenessProbe.TcpSocket.Port, pid)
	} else {
		log.Printf("launched '%s' deployment, pid=%d\n", c.Name, pid)
	}
	if pid == 0 {
		log.Printf("pid of %s is zero value", c.Name)
		return 0, fmt.Errorf("pid is zero")
	}
	return pid, nil
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
