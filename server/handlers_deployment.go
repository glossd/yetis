package server

import (
	"cmp"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"log"
	"slices"
	"strconv"
	"time"
)

func PostDeployment(spec common.DeploymentSpec) (*fetch.Empty, error) {
	spec, err := startDeploymentWithEnv(spec, false)
	if err != nil {
		return nil, err
	}

	startLivenessCheck(spec)
	return &fetch.Empty{}, nil
}

func startDeploymentWithEnv(spec common.DeploymentSpec, upsert bool) (common.DeploymentSpec, error) {
	spec, err := setDeploymentPortEnv(spec)
	if err != nil {
		return spec, err
	}
	spec = spec.WithDefaults().(common.DeploymentSpec)
	err = spec.Validate()
	if err != nil {
		return spec, fmt.Errorf("deployment %s spec is invalid: %s", spec.Name, err)
	}

	saved := saveDeployment(spec, upsert)
	if !saved {
		return spec, fmt.Errorf("deployment '%s' already exists", spec.Name)
	}
	pid, logPath, err := launchProcess(spec)
	if err != nil {
		deleteDeployment(spec.Name)
		return spec, err
	}
	err = updateDeployment(spec, pid, logPath, false)
	if err != nil {
		// For this to happen, delete must finish first before launching,
		// which is hard to imagine because start is asynchronous and delete is synchronous.
		log.Printf("Failed to update pid after launching process, pid=%d", pid)
		return spec, err
	}
	return spec, nil
}

func setDeploymentPortEnv(c common.DeploymentSpec) (common.DeploymentSpec, error) {
	deploymentPort, err := common.GetFreePort()
	if err != nil {
		return common.DeploymentSpec{}, fmt.Errorf("failed to assigned port: %s", err)
	}
	if c.LivenessProbe.TcpSocket.Port == 0 || isYetisPortUsed(c) {
		c.LivenessProbe.TcpSocket.Port = deploymentPort
	}

	var newEnvs []common.EnvVar
	for _, envVar := range c.Env {
		if envVar.Name == "YETIS_PORT" {
			// remove old YETIS_PORT env
			continue
		}
		if envVar.Value == "$YETIS_PORT" {
			newEnvs = append(newEnvs, common.EnvVar{Name: envVar.Name, Value: strconv.Itoa(deploymentPort)})
		} else {
			newEnvs = append(newEnvs, envVar)
		}
	}
	newEnvs = append(newEnvs, common.EnvVar{Name: "YETIS_PORT", Value: strconv.Itoa(deploymentPort)})
	c.Env = newEnvs
	return c, nil
}

func isYetisPortUsed(c common.DeploymentSpec) bool {
	return getDeploymentPort(c) == c.LivenessProbe.TcpSocket.Port
}

func getDeploymentPort(s common.DeploymentSpec) int {
	for _, envVar := range s.Env {
		if envVar.Name == "YETIS_PORT" {
			port, err := strconv.Atoi(envVar.Value)
			if err != nil {
				return 0
			}
			return port
		}
	}
	return 0
}

type DeploymentView struct {
	Name         string
	Status       string
	Pid          int
	Restarts     int
	Age          string
	Command      string
	LivenessPort int
}

func ListDeployment(_ fetch.Empty) ([]DeploymentView, error) {
	var res []DeploymentView
	rangeDeployments(func(name string, p deployment) {
		res = append(res, DeploymentView{
			Name:         name,
			Status:       p.status.String(),
			Pid:          p.pid,
			Restarts:     p.restarts,
			Age:          ageSince(p.createdAt),
			Command:      p.spec.Cmd,
			LivenessPort: p.spec.LivenessProbe.TcpSocket.Port,
		})
	})

	sortDeployments(res)
	return res, nil
}

func sortDeployments(res []DeploymentView) {
	slices.SortFunc(res, func(a, b DeploymentView) int {
		return cmp.Compare(a.Name, b.Name)
	})
}

func ageSince(t time.Time) string {
	age := time.Now().Sub(t)
	if age > 48*time.Hour {
		days := age.Hours() / 24
		return fmt.Sprintf("%.0fd", days)
	}
	if age > time.Hour {
		return fmt.Sprintf("%dh%dm", int(age.Hours()), int(age.Minutes())-(int(age.Hours())*60))
	}
	if age > time.Minute {
		return fmt.Sprintf("%dm%ds", int(age.Minutes()), int(age.Seconds())-(int(age.Minutes())*60))
	}
	return fmt.Sprintf("%ds", int(age.Seconds()))
}

type GetResponse struct {
	Pid      int
	Restarts int
	Status   string
	Age      string
	LogPath  string
	Spec     common.DeploymentSpec
}

func GetDeployment(r fetch.Request[fetch.Empty]) (*GetResponse, error) {
	name := r.PathValues["name"]
	if name == "" {
		return nil, fmt.Errorf("name can't be empty")
	}
	p, ok := deploymentStore.Load(name)
	if !ok {
		return nil, fmt.Errorf("name '%s' doesn't exist", name)
	}

	return &GetResponse{
		Pid:      p.pid,
		Restarts: p.restarts,
		Status:   p.status.String(),
		Age:      ageSince(p.createdAt),
		LogPath:  p.logPath,
		Spec:     p.spec,
	}, nil
}

func DeleteDeployment(r fetch.Request[fetch.Empty]) (*fetch.Empty, error) {
	name := r.PathValues["name"]
	if name == "" {
		return nil, fmt.Errorf(`name can't be empty`)
	}

	d, ok := getDeployment(name)
	if !ok {
		return nil, fmt.Errorf(`'%s' doesn't exist'`, name)
	}

	err := terminateProcess(r.Context, d)
	if err != nil {
		return nil, err
	}

	deleteDeployment(name)
	deleteLivenessCheck(name)
	return &fetch.Empty{}, nil
}

func RestartDeployment(r fetch.Request[fetch.Empty]) (*fetch.Empty, error) {
	name := r.PathValues["name"]
	if name == "" {
		return nil, fmt.Errorf(`name can't be empty`)
	}

	oldDeployment, ok := getDeployment(name)
	if !ok {
		return nil, fmt.Errorf(`deployment '%s' doesn't exist'`, name)
	}

	deleteLivenessCheck(name)
	var newSpec common.DeploymentSpec
	var err error
	if oldDeployment.spec.Strategy.Type == common.RollingUpdate {
		newSpec, err = startDeploymentWithEnv(oldDeployment.spec, true)
		if err != nil {
			return nil, fmt.Errorf("faield to start deployment: %s", err)
		}
		err = terminateProcess(r.Context, oldDeployment)
		if err != nil {
			return nil, fmt.Errorf("failed to terminate deployment's process: %s", err)
		}
	} else {
		err := terminateProcess(r.Context, oldDeployment)
		if err != nil {
			return nil, fmt.Errorf("failed to terminate deployment's process: %s", err)
		}
		newSpec, err = startDeploymentWithEnv(oldDeployment.spec, true)
		if err != nil {
			return nil, fmt.Errorf("faield to start deployment: %s", err)
		}
	}
	startLivenessCheck(newSpec)

	return &fetch.Empty{}, nil
}
