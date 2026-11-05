package itests

import (
	"bytes"
	"github.com/glossd/yetis/client"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"github.com/glossd/yetis/server"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRoot_RunDeploymentFromYetisConfig(t *testing.T) {
	skipIfNotRoot(t)

	configPath := pwd(t) + "/cfg/yetis-deployment-username.yaml"
	go server.Run(configPath)
	t.Cleanup(server.Stop)
	time.Sleep(time.Millisecond)
	errs := client.Apply(pwd(t) + "/specs/nc.yaml")
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}
	checkDeploymentRunning(t, "hello")

	ncPid, err := unix.GetPidByPort(27000)
	if err != nil {
		t.Fatalf("could not get pid of member 27000: %v", err)
	}

	owner, err := getUnixProcessOwner(ncPid)
	if err != nil {
		t.Fatalf("could not get owner of server: %v", err)
	}
	if owner == "root" {
		t.Errorf("owner of nc process is root")
	}
	expectedUsername := common.ReadServerConfig(configPath).Deployment.Username
	if owner != expectedUsername {
		t.Errorf("owner of nc process is the username from deployment, got=%s, want=%s", owner, expectedUsername)
	}
}

func getUnixProcessOwner(pid int) (string, error) {
	cmd := exec.Command("ps", "-o", "user=", "-p", strconv.Itoa(pid))

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

func skipIfNotRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.SkipNow()
	}
}
