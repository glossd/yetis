package server

import (
	"cmp"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/proxy"
	"log"
	"regexp"
	"slices"
	"strconv"
	"time"
)

func PostDeployment(req fetch.Request[common.DeploymentSpec]) error {
	spec := req.Body
	if spec.Strategy.Type == common.Recreate {
		if spec.Proxy.Port == 0 && spec.LivenessProbe.Port() == 0 {
			return fmt.Errorf("either livenessProbe.tcpSocket.port or proxy.port must be specified for Recreate strategy")
		}
	}
	if spec.Strategy.Type == common.RollingUpdate {
		// check the name was upgraded
		var err error
		deploymentStore.Range(func(name string, d deployment) bool {
			if spec.Name == rootNameForRollingUpdate(name) {
				err = fmt.Errorf("deployment '%s' has a rolling update name: %s", spec.Name, name)
				return false
			}
			return true
		})
		if err != nil {
			return err
		}
		if spec.LivenessProbe.Port() > 0 {
			return fmt.Errorf("livenessProxy.tcpSocket.port can't be specified with RollingUpdate strategy")
		}
		if spec.Proxy.Port == 0 {
			return fmt.Errorf("proxy.port must be specified with RollingUpdate strategy")
		}
	}

	if spec.Proxy.Port > 0 && spec.LivenessProbe.Port() > 0 {
		return fmt.Errorf("livenessProxy.tcpSocket.port can't be specified with proxy.port")
	}

	spec, err := setYetisPortEnv(spec.WithDefaults().(common.DeploymentSpec))
	if err != nil {
		return err
	}

	if spec.Proxy.Port > 0 {
		err := proxy.CreatePortForwarding(spec.Proxy.Port, spec.LivenessProbe.Port())
		if err != nil {
			return fmt.Errorf("failed to create proxy: %s", err)
		}
	}

	spec, err = startDeploymentWithEnv(spec, false, false)
	if err != nil {
		if spec.Proxy.Port > 0 {
			_ = proxy.DeletePortForwarding(spec.Proxy.Port, spec.LivenessProbe.Port())
		}
		return err
	}

	startLivenessCheck(spec)

	return nil
}

func startDeploymentWithEnv(spec common.DeploymentSpec, upsert, setYetisPort bool) (common.DeploymentSpec, error) {
	var err error
	if setYetisPort {
		spec, err = setYetisPortEnv(spec.WithDefaults().(common.DeploymentSpec))
		if err != nil {
			return spec, err
		}
	}
	err = spec.Validate()
	if err != nil {
		return spec, fmt.Errorf("deployment %s spec is invalid: %s", spec.Name, err)
	}

	saved := saveDeployment(spec, upsert)
	if !saved {
		return spec, fmt.Errorf("deployment '%s' already exists", spec.Name)
	}
	pid, logPath, err := launchProcess(spec, false)
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

const yetisPortEnv = "YETIS_PORT"

func setYetisPortEnv(c common.DeploymentSpec) (common.DeploymentSpec, error) {
	freePort, err := common.GetFreePort()
	if err != nil {
		return common.DeploymentSpec{}, fmt.Errorf("failed to assigned port: %s", err)
	}
	if c.LivenessProbe.Port() == 0 || isYetisPortUsed(c) {
		c.LivenessProbe.TcpSocket.Port = freePort
	}

	var newEnvs []common.EnvVar
	for _, envVar := range c.Env {
		if envVar.Name == yetisPortEnv {
			// remove old env
			continue
		}
		newEnvs = append(newEnvs, envVar)
	}
	newEnvs = append(newEnvs, common.EnvVar{Name: yetisPortEnv, Value: strconv.Itoa(freePort)})
	c.Env = newEnvs
	return c, nil
}

func isYetisPortUsed(c common.DeploymentSpec) bool {
	return c.YetisPort() == c.LivenessProbe.TcpSocket.Port
}

type DeploymentInfo struct {
	Name     string
	Status   string
	Pid      int
	Restarts int
	Age      string
	Command  string
	// Deprecated.
	LivenessPort int
	PortInfo     string
}

func ListDeployment() ([]DeploymentInfo, error) {
	var res []DeploymentInfo
	rangeDeployments(func(name string, p deployment) {
		portInfo := strconv.Itoa(p.spec.LivenessProbe.Port())
		if p.spec.Proxy.Port > 0 {
			portInfo = strconv.Itoa(p.spec.Proxy.Port) + " to " + strconv.Itoa(p.spec.LivenessProbe.Port())
		}
		res = append(res, DeploymentInfo{
			Name:         name,
			Status:       p.status.String(),
			Pid:          p.pid,
			Restarts:     p.restarts,
			Age:          ageSince(p.createdAt),
			Command:      p.spec.Cmd,
			LivenessPort: p.spec.LivenessProbe.Port(),
			PortInfo:     portInfo,
		})
	})

	sortDeployments(res)
	return res, nil
}

func sortDeployments(res []DeploymentInfo) {
	slices.SortFunc(res, func(a, b DeploymentInfo) int {
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

type DeploymentFullInfo struct {
	Pid      int
	Restarts int
	Status   string
	Age      string
	LogPath  string
	Spec     common.DeploymentSpec
}

func GetDeployment(r fetch.Request[fetch.Empty]) (*DeploymentFullInfo, error) {
	name := r.PathValues["name"]
	if name == "" {
		return nil, fmt.Errorf("name can't be empty")
	}
	p, ok := deploymentStore.Load(name)
	if !ok {
		return nil, fmt.Errorf("name '%s' doesn't exist", name)
	}

	return deploymentToInfo(p), nil
}

func deploymentToInfo(p deployment) *DeploymentFullInfo {
	return &DeploymentFullInfo{
		Pid:      p.pid,
		Restarts: p.restarts,
		Status:   p.status.String(),
		Age:      ageSince(p.createdAt),
		LogPath:  p.logPath,
		Spec:     p.spec,
	}
}

func DeleteDeployment(r fetch.Request[fetch.Empty]) error {
	name := r.PathValues["name"]
	if name == "" {
		return fmt.Errorf(`name can't be empty`)
	}

	d, ok := getDeployment(name)
	if !ok {
		return fmt.Errorf(`'%s' doesn't exist'`, name)
	}

	updateDeploymentStatus(name, Terminating)

	err := terminateProcess(r.Context, d)
	if err != nil {
		return err
	}

	deleteDeployment(name)
	deleteLivenessCheck(name)
	if d.spec.Proxy.Port > 0 {
		err := proxy.DeletePortForwarding(d.spec.Proxy.Port, d.spec.LivenessProbe.Port())
		if err != nil {
			log.Println("Failed to delete port forwarding:", err)
		}
	}
	log.Printf("Deleted deployment '%s'\n", name)
	return nil
}

func RestartDeployment(r fetch.Request[fetch.Empty]) error {
	name := r.PathValues["name"]
	if name == "" {
		return fmt.Errorf(`name can't be empty`)
	}

	oldDeployment, ok := getDeployment(name)
	if !ok {
		return fmt.Errorf(`deployment '%s' doesn't exist'`, name)
	}

	deleteLivenessCheck(name)
	var newSpec common.DeploymentSpec
	var err error
	if oldDeployment.spec.Strategy.Type == common.RollingUpdate {
		newSpec = oldDeployment.spec
		newSpec.Name = upgradeNameForRollingUpdate(newSpec.Name)
		newSpec, err = startDeploymentWithEnv(newSpec, false, true)
		if err != nil {
			return fmt.Errorf("rastart failed: the new rolling deployment of '%s' failed to start: %s", oldDeployment.spec.Name, err)
		}
		startLivenessCheck(newSpec)
		// check that the new deployment is healthy

		duration := 10000*time.Millisecond + newSpec.LivenessProbe.InitialDelayDuration() + time.Duration(newSpec.LivenessProbe.FailureThreshold)*newSpec.LivenessProbe.PeriodDuration()
		timeout := time.After(duration)
	loop:
		for {
			select {
			case <-timeout:
				// don't delete, need to see what went wrong.
				return fmt.Errorf("rastart failed: the new '%s' deployment isn't healthy: context deadline exceeded", newSpec.Name)
			default:
				depStatus, ok := getDeploymentStatus(newSpec.Name)
				if !ok {
					// shouldn't happen
					return fmt.Errorf("rastart failed: new '%s' deployment not found", oldDeployment.spec.Name)
				}
				if depStatus == Running {
					break loop
				}
			}
		}

		// point to the new port
		err := proxy.UpdatePortForwarding(oldDeployment.spec.Proxy.Port, oldDeployment.spec.LivenessProbe.Port(), newSpec.LivenessProbe.Port())
		if err != nil {
			return fmt.Errorf("started new deployment but failed to update proxy: %s", err)
		}

		// give it 50 millis in case the deployment doesn't have graceful shutdown
		time.Sleep(50 * time.Millisecond)

		// delete old deployment
		err = DeleteDeployment(fetch.Request[fetch.Empty]{Context: r.Context, PathValues: map[string]string{"name": oldDeployment.spec.Name}})
		if err != nil {
			return fmt.Errorf("failed to delete old deployment '%s': %s", oldDeployment.spec.Name, err)
		}

	} else {
		err := terminateProcess(r.Context, oldDeployment)
		if err != nil {
			return fmt.Errorf("failed to terminate deployment's process: %s", err)
		}
		newSpec, err = startDeploymentWithEnv(oldDeployment.spec, true, true)
		if err != nil {
			return fmt.Errorf("faield to start deployment: %s", err)
		}

		if oldDeployment.spec.Proxy.Port > 0 {
			err := proxy.UpdatePortForwarding(oldDeployment.spec.Proxy.Port, oldDeployment.spec.LivenessProbe.Port(), newSpec.LivenessProbe.Port())
			if err != nil {
				return fmt.Errorf("restarted deployment but failed to update proxy port: %s", err)
			}
		}

		startLivenessCheck(newSpec)
	}

	return nil
}

var rollingUpdatePattern = regexp.MustCompile(`^.*-(\d+)$`)

func upgradeNameForRollingUpdate(oldName string) string {
	matchPairs := rollingUpdatePattern.FindStringSubmatchIndex(oldName)
	if len(matchPairs) < 1 {
		return oldName + "-1"
	}
	idx := matchPairs[len(matchPairs)-2]
	numStr := oldName[idx:]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return oldName + "-1"
	}

	return oldName[:idx] + strconv.Itoa(num+1)
}

var rollingUpdateRootPattern = regexp.MustCompile(`^(.*)-\d+$`)

func rootNameForRollingUpdate(name string) string {
	matchPairs := rollingUpdateRootPattern.FindStringSubmatchIndex(name)
	if len(matchPairs) < 1 {
		return name
	}
	idx := matchPairs[len(matchPairs)-1]
	return name[:idx]
}
