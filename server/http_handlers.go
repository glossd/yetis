package server

import (
	"cmp"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"log"
	"slices"
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
	err := startDeployment(c)
	if err != nil {
		return err
	}
	runLivenessCheck(c, 2)
	return nil
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

type GetRequest struct {
	Name string
}

func Get(r fetch.Request[GetRequest]) (*GetResponse, error) {
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
