package server

import (
	"fmt"
	"github.com/glossd/yetis/common"
	"log"
	"sync"
	"time"
)

var serviceStore = common.Map[string, service]{}

var serviceWriteLock = sync.RWMutex{}

type service struct {
	pid        int
	status     ProcessStatus
	createdAt  time.Time
	spec       common.ServiceSpec
	targetPort int
	// port to send the new target port.
	updatePort int
}

func (s service) getPid() int {
	return s.pid
}

func (s service) getPort() int {
	return s.spec.Port
}

func firstSaveService(s common.ServiceSpec) error {
	serviceWriteLock.Lock()
	defer serviceWriteLock.Unlock()

	_, ok := serviceStore.Load(s.Selector.Name)
	if ok {
		return fmt.Errorf("service for '%s' already exists", s.Selector.Name)
	}
	serviceStore.Store(s.Selector.Name, service{
		createdAt: time.Now(),
		spec:      s,
	})
	return nil
}

func updateService(s common.ServiceSpec, pid int, status ProcessStatus, deploymentPort, httpPort int) error {
	serviceWriteLock.Lock()
	defer serviceWriteLock.Unlock()

	v, ok := serviceStore.Load(s.Selector.Name)
	if !ok {
		return fmt.Errorf("service for '%s' not found", s.Selector.Name)
	}

	v.pid = pid
	v.status = status
	v.targetPort = deploymentPort
	v.updatePort = httpPort
	serviceStore.Store(s.Selector.Name, v)
	return nil
}

func updateServiceStatus(name string, status ProcessStatus) {
	serviceWriteLock.Lock()
	defer serviceWriteLock.Unlock()
	v, ok := serviceStore.Load(name)
	if !ok {
		log.Printf("tried to update status but service for '%s' doesn't exist\n", name)
		return
	}
	v.status = status
	serviceStore.Store(name, v)
}

func servicesNum() int {
	var num int
	serviceStore.Range(func(name string, p service) bool {
		num++
		return true
	})
	return num
}
