package server

import (
	"github.com/glossd/yetis/common"
	"testing"
	"time"
)

func TestLivenessFailed(t *testing.T) {
	config := common.DeploymentSpec{
		Name:   "liveness",
		Cmd:    "echo 'Liveness Test'",
		Logdir: "stdout",
		LivenessProbe: common.Probe{
			TcpSocket:           common.TcpSocket{Port: 27000},
			InitialDelaySeconds: 0.01,
			FailureThreshold:    2,
			SuccessThreshold:    1,
		},
	}

	_, err := startDeploymentWithEnv(config, false)
	assert(t, err, nil)
	defer deleteDeployment(config.Name)
	isPortOpenMock = BoolPtr(true)
	defer func() { isPortOpenMock = nil }()
	assertD(t, Pending, 0)
	heartbeat(config.Name, 2)
	time.Sleep(time.Millisecond)
	assertD(t, Running, 0)
	heartbeat(config.Name, 2)
	assertD(t, Running, 0)
	isPortOpenMock = BoolPtr(false)
	heartbeat(config.Name, 2)
	assertD(t, Running, 0)
	heartbeat(config.Name, 2)
	assertD(t, Pending, 1)
	heartbeat(config.Name, 2)
	assertD(t, Pending, 1)
	heartbeat(config.Name, 2)
	assertD(t, Pending, 2)
	heartbeat(config.Name, 2)
	assertD(t, Pending, 2)
	heartbeat(config.Name, 2)
	assertD(t, Failed, 2)
}

func TestLivenessResurrection(t *testing.T) {
	config := common.DeploymentSpec{
		Name:   "liveness",
		Cmd:    "echo 'Liveness Test'",
		Logdir: "stdout",
		LivenessProbe: common.Probe{
			TcpSocket:           common.TcpSocket{Port: 27000},
			InitialDelaySeconds: 0.01,
			FailureThreshold:    2,
			SuccessThreshold:    1,
		},
	}
	_, err := startDeploymentWithEnv(config, false)
	assert(t, err, nil)
	defer deleteDeployment(config.Name)
	isPortOpenMock = BoolPtr(true)
	defer func() { isPortOpenMock = nil }()
	assertD(t, Pending, 0)
	heartbeat(config.Name, 2)
	assertD(t, Running, 0)
	heartbeat(config.Name, 2)
	assertD(t, Running, 0)
	isPortOpenMock = BoolPtr(false)
	heartbeat(config.Name, 2)
	assertD(t, Running, 0)
	heartbeat(config.Name, 2)
	assertD(t, Pending, 1)
	isPortOpenMock = BoolPtr(true)
	heartbeat(config.Name, 2)
	assertD(t, Running, 1)
}

func assertD(t *testing.T, status ProcessStatus, restarts int) {
	t.Helper()
	d, ok := getDeployment("liveness")
	assert(t, ok, true)
	assert(t, d.status, status)
	assert(t, d.restarts, restarts)
}

func BoolPtr(b bool) *bool {
	return &b
}
