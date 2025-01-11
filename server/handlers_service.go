package server

import (
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/proxy"
	"log"
	"path/filepath"
	"time"
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
			DeploymentPort: v.targetPort,
			SelectorName:   v.spec.Selector.Name,
		})
		return true
	})
	return res, nil
}

type GetServiceResponse struct {
	Pid            int
	Port           int
	SelectorName   string
	DeploymentPort int
}

func GetService(in fetch.Request[fetch.Empty]) (*GetServiceResponse, error) {
	name := in.PathValues["name"]
	ser, ok := serviceStore.Load(name)
	if !ok {
		return nil, serviceNotFound(name)
	}
	return &GetServiceResponse{
		Pid:            ser.pid,
		Port:           ser.spec.Port,
		DeploymentPort: ser.targetPort,
		SelectorName:   ser.spec.Selector.Name,
	}, nil
}
func PostService(spec common.ServiceSpec) (*fetch.Empty, error) {
	dep, ok := getDeployment(spec.Selector.Name)
	if !ok {
		return nil, fmt.Errorf("selected deployment '%s' doesn't exist", spec.Selector.Name)
	}
	if common.IsPortOpenTimeout(spec.Port, 100*time.Millisecond) {
		return nil, fmt.Errorf("service port %d already occupied", spec.Port)
	}
	err := firstSaveService(spec)
	if err != nil {
		return nil, err
	}
	deploymentPort := getDeploymentPort(dep.spec)
	logdir := "/tmp"
	if spec.Logdir != "" {
		logdir = spec.Logdir
	}
	logpath := filepath.Join(logdir, fmt.Sprintf("service-to-%s.log", spec.Selector.Name))
	pid, httpPort, err := proxy.Exec(spec.Port, deploymentPort, logpath)
	if err != nil {
		return nil, fmt.Errorf("failed to start service: %s", err)
	}
	log.Printf("launched service for %s deployment on port %d to port %d with updatePort %d", spec.Selector.Name, spec.Port, deploymentPort, httpPort)
	err = updateService(spec, pid, Pending, deploymentPort, httpPort)
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
		return nil, serviceNotFound(name)
	}
	err := terminateProcess(in.Context, ser)
	if err != nil {
		return nil, fmt.Errorf("service for '%s' failed to terminate: %s", name, err)
	}
	serviceStore.Delete(name)
	return nil, nil
}

func UpdateServiceTargetPort(in fetch.Request[int]) error {
	name := in.PathValues["name"]
	serv, ok := serviceStore.Load(name)
	if !ok {
		return serviceNotFound(name)
	}

	newTargetPort := in.Body
	_, err := fetch.Post[fetch.Empty](fmt.Sprintf("http://localhost:%d/update", serv.updatePort), newTargetPort)
	if err != nil {
		return fmt.Errorf("failed to update port: %s", err)
	}
	serv.targetPort = newTargetPort
	serviceStore.Store(name, serv)

	return nil
}

func serviceNotFound(name string) *fetch.Error {
	return &fetch.Error{Status: 404, Msg: fmt.Sprintf("service for '%s' deployment not found", name)}
}
