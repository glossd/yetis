package proxy

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

//go:embed cmd/main
var binary []byte

func Exec(port, portTo int) (int, error) {
	file, err := os.Create("/tmp/yetis-proxy")
	if err != nil {
		return 0, fmt.Errorf("failed to create/open yetis-proxy file: %s", err)
	}
	_, err = file.Write(binary)
	if err != nil {
		return 0, fmt.Errorf("failed to write to yetis-proxy file: %s", err)
	}
	err = exec.Command("chmod", "+x", "/tmp/yetis-proxy").Run()
	if err != nil {
		return 0, fmt.Errorf("failed to make yetis-proxy executable: %s", err)
	}
	cmd := exec.Command("/tmp/yetis-proxy", strconv.Itoa(port), strconv.Itoa(portTo))
	err = cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to start command: %s", err)
	}
	if cmd.Process.Pid == 0 {
		return 0, fmt.Errorf("pid is 0")
	}
	return cmd.Process.Pid, nil
}
