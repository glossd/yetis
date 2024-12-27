package proxy

import (
	_ "embed"
	"fmt"
	"github.com/glossd/yetis/common"
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

//go:embed cmd/main
var binary []byte

func Exec(port, targetPort int) (int, int, error) {
	const filePath = "/tmp/yetis-proxy"

	if !proxyFileExists(filePath) {
		log.Println("tcp proxy file doesn't exist, creating one...")
		err := createYetisProxyFile(filePath)
		if err != nil {
			return 0, 0, err
		}
	}

	httpPort, err := common.GetFreePort()
	if err != nil {
		return 0, 0, err
	}
	cmd := exec.Command(filePath, strconv.Itoa(port), strconv.Itoa(targetPort), strconv.Itoa(httpPort))
	err = cmd.Start()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to start command: %s", err)
	}
	if cmd.Process.Pid == 0 {
		return 0, 0, fmt.Errorf("pid is 0")
	}
	return cmd.Process.Pid, httpPort, nil
}

func proxyFileExists(filePath string) bool {
	fi, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return fi.Size() == int64(len(binary))
}

var createMutex sync.Mutex

func createYetisProxyFile(filePath string) error {
	createMutex.Lock()
	defer createMutex.Unlock()
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create/open yetis-proxy file: %s", err)
	}
	_, err = file.Write(binary)
	if err != nil {
		return fmt.Errorf("failed to write to yetis-proxy file: %s", err)
	}
	err = file.Close()
	if err != nil {
		return fmt.Errorf("failed to close yetis-proxy file writer: %s", err)
	}
	err = exec.Command("chmod", "+x", filePath).Run()
	if err != nil {
		return fmt.Errorf("failed to make yetis-proxy executable: %s", err)
	}
	return nil
}
