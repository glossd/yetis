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
