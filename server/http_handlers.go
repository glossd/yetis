package server

import (
	"cmp"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"github.com/glossd/yetis/proxy"
	"log"
	"slices"
	"strconv"
	"time"
)

type PostResponse struct {
	Success int
	Failure int
	Error   string
}

func Post(confs []common.Config) (PostResponse, error) {
	var errs []error
	for _, conf := range confs {
		err := applyConfig(conf)
		if err != nil {
			errs = append(errs, err)
		}
	}
	var errStr string
	for _, err := range errs {
		errStr = errStr + err.Error() + "\n"
	}

	return PostResponse{
		Success: len(confs) - len(errs),
		Failure: len(errs),
		Error:   errStr,
	}, nil
}

func applyConfig(c common.Config) error {
	if c.Spec.Proxy.Port > 0 {
		var err error
		c, err = setDeploymentPortEnv(c)
		if err != nil {
			return err
		}
	}

	err := startDeployment(c)
	if err != nil {
		return err
	}

	runLivenessCheck(c, 2)
	return nil
}

func setDeploymentPortEnv(c common.Config) (common.Config, error) {
	deploymentPort, err := common.GetFreePort()
	if err != nil {
		return common.Config{}, fmt.Errorf("proxy is configured, failed to assigned port: %s", err)
	}
	c.Spec.LivenessProbe.TcpSocket.Port = deploymentPort
	newEnvs := []common.EnvVar{{Name: "YETIS_PORT", Value: strconv.Itoa(deploymentPort)}}
	for _, envVar := range c.Spec.Env {
		if envVar.Value == "$YETIS_PORT" {
			newEnvs = append(newEnvs, common.EnvVar{Name: envVar.Name, Value: strconv.Itoa(deploymentPort)})
		} else {
			newEnvs = append(newEnvs, envVar)
		}
	}
	c.Spec.Env = newEnvs
	return c, nil
}

func startDeployment(c common.Config) error {
	ok := saveDeployment(c, 0)
	if !ok {
		return fmt.Errorf("deployment '%s' already exists", c.Spec.Name)
	}
	pid, logPath, err := launchProcess(c)
	if err != nil {
		deleteDeployment(c.Spec.Name)
		return err
	}
	err = updateDeployment(c.Spec.Name, pid, logPath, false)
	if err != nil {
		// For this to happen, delete must finish first before launching,
		// which is hard to imagine because start is asynchronous and delete is synchronous.
		log.Printf("Failed to update pid after launching process, pid=%d", pid)
		return err
	}
	return nil
}

func startProxy(c common.Config) error {
	// saveProxy
	pid, err := proxy.Exec(c.Spec.Proxy.Port, c.Spec.LivenessProbe.TcpSocket.Port)
	if err != nil {
		return fmt.Errorf("failed to start proxy for %s: %s", c.Spec.Name, err)
	}
	// updateProxy
	return nil
}

type DeploymentView struct {
	Name     string
	Status   string
	Pid      int
	Restarts int
	Age      string
	Command  string
}

func List(_ fetch.Empty) ([]DeploymentView, error) {
	var res []DeploymentView
	rangeDeployments(func(name string, p deployment) {
		res = append(res, DeploymentView{
			Name:     name,
			Status:   p.status.String(),
			Pid:      p.pid,
			Restarts: p.restarts,
			Age:      ageSince(p.createdAt),
			Command:  p.config.Spec.Cmd,
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
	Config   common.Config
}

func Get(r fetch.Request[fetch.Empty]) (*GetResponse, error) {
	name := r.PathValues["name"]
	if name == "" {
		return nil, fmt.Errorf("name can't be empty")
	}
	p, ok := getDeployment(name)
	if !ok {
		return nil, fmt.Errorf("name '%s' doesn't exist", name)
	}

	return &GetResponse{
		Pid:      p.pid,
		Restarts: p.restarts,
		Status:   p.status.String(),
		Age:      ageSince(p.createdAt),
		LogPath:  p.logPath,
		Config:   p.config,
	}, nil
}

func Delete(r fetch.Request[fetch.Empty]) (*fetch.Empty, error) {
	name := r.PathValues["name"]
	if name == "" {
		return nil, fmt.Errorf(`name can't be empty`)
	}

	d, ok := getDeployment(name)
	if !ok {
		return nil, fmt.Errorf(`'%s' doesn't exist'`, name)
	}

	if d.pid != 0 {
		err := unix.TerminateProcess(r.Context, d.pid)
		if err != nil {
			return nil, err
		}
	}
	deleteDeployment(name)
	return &fetch.Empty{}, nil
}

type InfoResponse struct {
	Version             string
	NumberOfDeployments int
}

func Info(_ fetch.Empty) (*InfoResponse, error) {
	return &InfoResponse{
		Version:             common.YetisVersion,
		NumberOfDeployments: deploymentsNum(),
	}, nil
}
