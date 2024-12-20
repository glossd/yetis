package server

import (
	"fmt"
	"github.com/glossd/yetis/common"
	"log"
	"sync"
	"time"
)

// todo presist in sqlite
// name->pid
var deploymentStore = common.Map[string, deployment]{}

type resource interface {
	getPort() int
	getPid() int
}

type deployment struct {
	pid       int
	logPath   string
	restarts  int
	status    ProcessStatus
	createdAt time.Time
	spec      common.DeploymentSpec
}

func (d deployment) getPid() int {
	return d.pid
}

func (d deployment) getPort() int {
	return d.spec.LivenessProbe.TcpSocket.Port
}

type ProcessStatus int

const (
	Pending ProcessStatus = iota
	Running
	Failed
	Terminating
)

var processStatusMap = map[ProcessStatus]string{
	Pending:     "Pending",
	Running:     "Running",
	Failed:      "Failed",
	Terminating: "Terminating",
}

func (pc ProcessStatus) String() string {
	return processStatusMap[pc]
}

var writeLock sync.Mutex

func firstSaveDeployment(c common.DeploymentSpec) bool {
	writeLock.Lock()
	defer writeLock.Unlock()
	_, ok := deploymentStore.Load(c.Name)
	if ok {
		return false
	}
	deploymentStore.Store(c.Name, deployment{createdAt: time.Now(), spec: c})
	return true
}

func updateDeployment(s common.DeploymentSpec, pid int, logPath string, incRestarts bool) error {
	writeLock.Lock()
	defer writeLock.Unlock()
	d, ok := deploymentStore.Load(s.Name)
	if !ok {
		return fmt.Errorf("deployment %s doesn't exist", s.Name)
	}
	d.pid = pid
	d.logPath = logPath
	if incRestarts {
		d.restarts++
	}
	d.spec = s
	deploymentStore.Store(s.Name, d)
	return nil
}

func updateDeploymentStatus(name string, status ProcessStatus) {
	writeLock.Lock()
	defer writeLock.Unlock()
	v, ok := deploymentStore.Load(name)
	if !ok {
		log.Printf("tried to update status but deployment '%s' doesn't exist\n", name)
		return
	}
	v.status = status
	deploymentStore.Store(name, v)
}

func getDeployment(name string) (deployment, bool) {
	return deploymentStore.Load(name)
}

func deleteDeployment(name string) {
	deploymentStore.Delete(name)
}

func rangeDeployments(f func(name string, p deployment)) {
	deploymentStore.Range(func(k string, v deployment) bool {
		f(k, v)
		return true
	})
}

func deploymentsNum() int {
	var num int
	rangeDeployments(func(name string, p deployment) {
		num++
	})
	return num
}
