package proxy

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
)

//go:embed cmd/main
var binary []byte

func Exec(port, portTo int) (int, error) {
	filePath := "/tmp/yetis-proxy"

	if !proxyFileExists(filePath) {
		log.Println("tcp proxy file doesn't exist, creating one...")
		err := createYetisProxyFile(filePath)
		if err != nil {
			return 0, err
		}
	}
	cmd := exec.Command(filePath, strconv.Itoa(port), strconv.Itoa(portTo))
	err := cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to start command: %s", err)
	}
	if cmd.Process.Pid == 0 {
		return 0, fmt.Errorf("pid is 0")
	}
	return cmd.Process.Pid, nil
}

func proxyFileExists(filePath string) bool {
	fi, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return fi.Size() == int64(len(binary))
}

func createYetisProxyFile(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create/open yetis-proxy file: %s", err)
	}
	_, err = file.Write(binary)
	if err != nil {
		return fmt.Errorf("failed to write to yetis-proxy file: %s", err)
	}
	err = exec.Command("chmod", "+x", filePath).Run()
	if err != nil {
		return fmt.Errorf("failed to make yetis-proxy executable: %s", err)
	}
	return nil
}
