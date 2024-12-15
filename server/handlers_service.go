package server

import (
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"github.com/glossd/yetis/proxy"
	"log"
)

type ServiceView struct {
	Pid            int
	Port           int
	SelectorName   string
	DeploymentPort int
}

func ListService(_ fetch.Empty) ([]ServiceView, error) {
	var res []ServiceView
	serviceStore.Range(func(k string, v service) bool {
		res = append(res, ServiceView{
			Pid:            v.pid,
			Port:           v.spec.Port,
			DeploymentPort: v.deploymentPort,
			SelectorName:   v.spec.Selector.Name,
		})
		return true
	})
	return res, nil
}

type GetServiceResponse struct {
	Pid          int
	SelectorName string
}

func GetService(in fetch.Request[fetch.Empty]) (*GetServiceResponse, error) {
	name := in.PathValues["name"]
	ser, ok := serviceStore.Load(name)
	if !ok {
		return nil, fmt.Errorf("service for '%s' not found", name)
	}
	return &GetServiceResponse{
		Pid:          ser.pid,
		SelectorName: ser.spec.Selector.Name,
	}, nil
}
func PostService(spec common.ServiceSpec) (*fetch.Empty, error) {
	dep, ok := getDeployment(spec.Selector.Name)
	if !ok {
		return nil, fmt.Errorf("selected deployment '%s' doesn't exist", spec.Selector.Name)
	}
	err := firstSaveService(spec)
	if err != nil {
		return nil, err
	}
	deploymentPort := getDeploymentPort(dep.spec)
	pid, err := proxy.Exec(spec.Port, deploymentPort)
	if err != nil {
		return nil, fmt.Errorf("failed to start service: %s", err)
	}
	log.Printf("launched service for %s deployment on port %d to port %d", spec.Selector.Name, spec.Port, deploymentPort)
	err = updateService(spec, pid, Pending, deploymentPort)
	if err != nil {
		return nil, err
	}

	// todo runLiveness()

	return nil, nil
}
func DeleteService(in fetch.Request[fetch.Empty]) (*fetch.Empty, error) {
	name := in.PathValues["name"]
	ser, ok := serviceStore.Load(name)
	if !ok {
		return nil, fmt.Errorf("service for '%s' not found", name)
	}
	err := unix.TerminateProcess(in.Context, ser.pid)
	if err != nil {
		return nil, fmt.Errorf("service for '%s' failed to terminate: %s", name, err)
	}
	return nil, nil
}
