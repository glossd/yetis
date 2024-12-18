package server

import (
	"fmt"
	"github.com/glossd/yetis/common"
	"sync"
	"time"
)

var serviceStore = common.Map[string, service]{}

var serviceWriteLock = sync.RWMutex{}

type service struct {
	pid            int
	status         ProcessStatus
	createdAt      time.Time
	spec           common.ServiceSpec
	deploymentPort int
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

func updateService(s common.ServiceSpec, pid int, status ProcessStatus, deploymentPort int) error {
	serviceWriteLock.Lock()
	defer serviceWriteLock.Unlock()

	v, ok := serviceStore.Load(s.Selector.Name)
	if !ok {
		return fmt.Errorf("service for '%s' not found", s.Selector.Name)
	}

	v.pid = pid
	v.status = status
	v.deploymentPort = deploymentPort
	serviceStore.Store(s.Selector.Name, v)
	return nil
}
