package server

import (
	"github.com/glossd/yetis/common"
	"testing"
	"time"
)

func TestRunLivenessToFailed(t *testing.T) {
	config := common.DeploymentSpec{
		Name:   "liveness",
		Cmd:    "echo 'Liveness Test'",
		Logdir: "stdout",
		LivenessProbe: common.Probe{
			TcpSocket:           common.TcpSocket{Port: 27000},
			InitialDelaySeconds: 0,
			FailureThreshold:    2,
			SuccessThreshold:    1,
		},
	}
	var ticker = make(chan time.Time)
	tick := func() {
		ticker <- time.Now()
		time.Sleep(50 * time.Millisecond)
	}
	err := startDeployment(config)
	assert(t, err, nil)
	defer deleteDeployment(config.Name)
	isPortOpenMock = BoolPtr(true)
	defer func() { isPortOpenMock = nil }()
	assertD(t, Pending, 0)
	tickerMock = &ticker
	defer func() { tickerMock = nil }()
	runLivenessCheck(config, 2)
	time.Sleep(time.Millisecond)

	assertD(t, Running, 0)
	tick()
	assertD(t, Running, 0)
	isPortOpenMock = BoolPtr(false)
	tick()
	assertD(t, Running, 0)
	tick()
	assertD(t, Pending, 1)
	tick()
	assertD(t, Pending, 1)
	tick()
	assertD(t, Pending, 2)
	tick()
	assertD(t, Pending, 2)
	tick()
	assertD(t, Failed, 2)
}

func TestRunLivenessSuccess(t *testing.T) {
	config := common.DeploymentSpec{
		Name:   "liveness",
		Cmd:    "echo 'Liveness Test'",
		Logdir: "stdout",
		LivenessProbe: common.Probe{
			TcpSocket:           common.TcpSocket{Port: 27000},
			InitialDelaySeconds: 0,
			FailureThreshold:    2,
			SuccessThreshold:    1,
		},
	}
	var ticker = make(chan time.Time)
	tick := func() {
		ticker <- time.Now()
		time.Sleep(50 * time.Millisecond)
	}
	err := startDeployment(config)
	assert(t, err, nil)
	defer deleteDeployment(config.Name)
	isPortOpenMock = BoolPtr(true)
	defer func() { isPortOpenMock = nil }()
	assertD(t, Pending, 0)
	tickerMock = &ticker
	defer func() { tickerMock = nil }()
	runLivenessCheck(config, 2)
	time.Sleep(time.Millisecond)
	assertD(t, Running, 0)
	tick()
	assertD(t, Running, 0)
	isPortOpenMock = BoolPtr(false)
	tick()
	assertD(t, Running, 0)
	tick()
	assertD(t, Pending, 1)
	isPortOpenMock = BoolPtr(true)
	tick()
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
