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
var deploymentStore = sync.Map{}

type deployment struct {
	pid       int
	logPath   string
	restarts  int
	status    DeploymentStatus
	createdAt time.Time
	config    common.Config
}

type DeploymentStatus int

const (
	Pending DeploymentStatus = iota
	Running
	Failed
	Terminating
)

var processStatusMap = map[DeploymentStatus]string{
	Pending:     "Pending",
	Running:     "Running",
	Failed:      "Failed",
	Terminating: "Terminating",
}

func (pc DeploymentStatus) String() string {
	return processStatusMap[pc]
}

var writeLock sync.Mutex

func saveDeployment(c common.Config, pid int) bool {
	writeLock.Lock()
	defer writeLock.Unlock()
	_, ok := getDeployment(c.Spec.Name)
	if ok {
		return false
	}
	deploymentStore.Store(c.Spec.Name, deployment{pid: pid, restarts: 0, createdAt: time.Now(), config: c})
	return true
}

func updateDeployment(name string, pid int, logPath string, incRestarts bool) error {
	writeLock.Lock()
	defer writeLock.Unlock()
	d, ok := getDeployment(name)
	if !ok {
		return fmt.Errorf("deployment %s doesn't exist", name)
	}
	d.pid = pid
	d.logPath = logPath
	if incRestarts {
		d.restarts++
	}
	deploymentStore.Store(name, d)
	return nil
}

func updateDeploymentStatus(name string, status DeploymentStatus) {
	writeLock.Lock()
	defer writeLock.Unlock()
	v, ok := deploymentStore.Load(name)
	if !ok {
		log.Printf("tried to update status but deployment '%s' doesn't exist\n", name)
		return
	}
	p := v.(deployment)
	p.status = status
	deploymentStore.Store(name, p)
}

func getDeploymentPid(name string) int {
	v, ok := getDeployment(name)
	if !ok {
		return 0
	}
	return v.pid
}

func getDeployment(name string) (deployment, bool) {
	v, ok := deploymentStore.Load(name)
	if !ok {
		return deployment{}, false
	}
	return v.(deployment), true
}

func deleteDeployment(name string) {
	deploymentStore.Delete(name)
}

func rangeDeployments(f func(name string, p deployment)) {
	deploymentStore.Range(func(key, value any) bool {
		k := key.(string)
		v, ok := value.(deployment)
		if !ok {
			return true
		}
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
