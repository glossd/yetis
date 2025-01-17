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
	Status         string
	Port           int
	SelectorName   string
	DeploymentPort int
	UpdatePort     int
}

func ListService(_ fetch.Empty) ([]ServiceView, error) {
	var res []ServiceView
	serviceStore.Range(func(k string, v service) bool {
		res = append(res, ServiceView{
			Pid:            v.pid,
			Status:         v.status.String(),
			Port:           v.spec.Port,
			DeploymentPort: v.targetPort,
			SelectorName:   v.spec.Selector.Name,
			UpdatePort:     v.updatePort,
		})
		return true
	})
	return res, nil
}

type GetServiceResponse struct {
	ServiceView
}

func GetService(in fetch.Request[fetch.Empty]) (*GetServiceResponse, error) {
	name := in.PathValues["name"]
	ser, ok := serviceStore.Load(name)
	if !ok {
		return nil, serviceNotFound(name)
	}
	return &GetServiceResponse{ServiceView: ServiceView{
		Pid:            ser.pid,
		Status:         ser.status.String(),
		Port:           ser.spec.Port,
		DeploymentPort: ser.targetPort,
		SelectorName:   ser.spec.Selector.Name,
		UpdatePort:     ser.updatePort,
	}}, nil
}
func PostService(spec common.ServiceSpec) (*fetch.Empty, error) {
	dep, ok := getDeployment(spec.Selector.Name)
	if !ok {
		return nil, fmt.Errorf("selected deployment '%s' doesn't exist", spec.Selector.Name)
	}
	if common.IsPortOpenTimeout(spec.Port, 100*time.Millisecond) {
		return nil, fmt.Errorf("port %d is already occupied", spec.Port)
	}
	if spec.Logdir == "" {
		spec.Logdir = "/tmp"
	}
	if spec.LivenessProbe.InitialDelaySeconds == 0 {
		spec.LivenessProbe.InitialDelaySeconds = 5
	}
	err := firstSaveService(spec)
	if err != nil {
		return nil, err
	}
	deploymentPort := dep.spec.YetisPort()
	pid, httpPort, err := proxy.Exec(spec.Port, deploymentPort, getServiceLogPath(spec))
	if err != nil {
		return nil, fmt.Errorf("failed to start service: %s", err)
	}
	log.Printf("launched service for %s deployment on port %d to port %d with updatePort %d", spec.Selector.Name, spec.Port, deploymentPort, httpPort)
	err = updateService(spec, pid, Pending, deploymentPort, httpPort)
	if err != nil {
		return nil, err
	}

	// liveness check
	time.AfterFunc(spec.LivenessProbe.InitialDelayDuration(), func() {
		startLivenessForService(spec)
	})

	return nil, nil
}

func getServiceLogPath(spec common.ServiceSpec) string {
	return filepath.Join(spec.Logdir, fmt.Sprintf("service-to-%s.log", spec.Selector.Name))
}

func startLivenessForService(spec common.ServiceSpec) {
	name := spec.Selector.Name
	for {
		ser, ok := serviceStore.Load(name)
		if !ok {
			break
		}
		if common.IsPortOpenRetry(ser.spec.Port, time.Second, 3) { // basically 3 failureThreshold
			updateServiceStatus(name, Running)
			time.Sleep(100 * time.Millisecond)
		} else {
			updateServiceStatus(name, Failed)
			log.Printf("Port %d of service for %s was closed\n", ser.spec.Port, name)
			// try to restart it
			dep, ok := getDeployment(name)
			if !ok {
				// what to do?
				continue
			}
			deploymentPort := dep.spec.YetisPort()
			pid, httpPort, err := proxy.Exec(spec.Port, deploymentPort, getServiceLogPath(ser.spec))
			if err != nil {
				log.Printf("Failed to restart service for '%s': %s\n", name, err)
				break
			}
			err = updateService(spec, pid, Pending, deploymentPort, httpPort)
			if err != nil {
				// shouldn't happen
				log.Printf("Failed to update service for '%s': %s\n", name, err)
				break
			}
			// another liveness check
			time.AfterFunc(5*time.Second, func() {
				startLivenessForService(spec)
			})
			break
		}
	}
}

func DeleteService(in fetch.Request[fetch.Empty]) (*fetch.Empty, error) {
	name := in.PathValues["name"]
	ser, ok := serviceStore.Load(name)
	if !ok {
		return nil, serviceNotFound(name)
	}
	// todo not being terminated in PG
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
	oldTargetPort := serv.targetPort
	serv.targetPort = newTargetPort
	serviceStore.Store(name, serv)
	log.Printf("Updated service for '%s' target port, old=%d, new=%d", name, oldTargetPort, newTargetPort)

	return nil
}

func serviceNotFound(name string) *fetch.Error {
	return &fetch.Error{Status: 404, Msg: fmt.Sprintf("service for '%s' deployment not found", name)}
}
