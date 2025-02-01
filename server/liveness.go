package server

import (
	"context"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/proxy"
	"log"
	"time"
)

// name -> Threshold
var thresholdMap = common.Map[string, Threshold]{}
var livenessMap = common.Map[string, chan bool]{}

type Threshold struct {
	SuccessCount int
	FailureCount int
}

const defaultRestartLimit = 2

// Non-blocking
func startLivenessCheck(c common.DeploymentSpec) {
	runLivenessCheck(c.Name, c.LivenessProbe.InitialDelayDuration(), c.LivenessProbe.PeriodDuration(), defaultRestartLimit, nil)
}

// Non-blocking.
func runLivenessCheck(name string, init, period time.Duration, restartsLimit int, stop chan bool) {
	if stop == nil {
		stop = make(chan bool)
	}
	livenessMap.Store(name, stop)
	cleanUp := func() {
		livenessMap.Delete(name)
		thresholdMap.Delete(name)
	}
	go func() {
		select {
		case <-stop:
			cleanUp()
			return
		case <-time.After(init):
			var ticker = time.NewTicker(period).C

			// check instantly
			if t := heartbeat(name, restartsLimit); t == dead {
				cleanUp()
				return
			}
			for {
				select {
				case <-stop:
					cleanUp()
					return
				case <-ticker:
					switch heartbeat(name, restartsLimit) {
					case dead:
						cleanUp()
						return
					case tryAgain:
						select {
						case <-stop:
							cleanUp()
							return
						case <-time.After(time.Duration(restartsLimit/2) * time.Minute):
							runLivenessCheck(name, init, period, restartsLimit*2, stop)
							return
						}
					}
				}
			}
		}
	}()
}

// Blocking
// * it seems timeout happens during TestLivenessRestart on server shutdown.
func deleteLivenessCheck(name string) bool {
	v, ok := livenessMap.Load(name)
	if ok {
		select {
		case v <- true:
			close(v)
			return true
		case <-time.After(3 * time.Second):
			log.Printf("delete liveness %s timeout\n", name)
			return false
		}
	}
	return false
}

type heartbeatResult int

const (
	dead = iota
	tryAgain
	alive
)

func heartbeat(deploymentName string, restartsLimit int) heartbeatResult {
	dep, ok := getDeployment(deploymentName)
	if !ok {
		// release go routine for GC
		return dead
	}
	if dep.status == Terminating {
		return alive
	}

	var port = dep.spec.LivenessProbe.Port()
	// Remove 10 milliseconds for everything to process and wait for the new tick.
	portOpen := isPortOpen(port, dep.spec.LivenessProbe.PeriodDuration()-10*time.Millisecond)
	tsh, ok := thresholdMap.Load(dep.spec.Name)
	if !ok {
		tsh = Threshold{}
	}
	if portOpen {
		tsh.FailureCount = 0
		tsh.SuccessCount++
	} else {
		tsh.FailureCount++
		tsh.SuccessCount = 0
	}
	thresholdMap.Store(dep.spec.Name, tsh)

	if tsh.FailureCount >= dep.spec.LivenessProbe.FailureThreshold {
		p, ok := getDeployment(dep.spec.Name)
		oldSpec := p.spec
		if !ok {
			log.Printf("Deployment '%s' reached failure threshold, but it doesn't exist\n", oldSpec.Name)
			return dead
		}
		if p.restarts >= restartsLimit {
			updateDeploymentStatus(oldSpec.Name, Failed)
			thresholdMap.Delete(oldSpec.Name)
			AlertFail(oldSpec.Name)
			return tryAgain
		}
		log.Printf("Restarting '%s' deployment, failureThreshold was reached\n", oldSpec.Name)
		updateDeploymentStatus(oldSpec.Name, Terminating)
		ctx, cancelCtx := context.WithTimeout(context.Background(), oldSpec.LivenessProbe.PeriodDuration())
		defer cancelCtx()
		err := terminateProcess(ctx, p.pid)
		if err != nil {
			log.Printf("failed to terminate process, deployment=%s, pid=%d\n", oldSpec.Name, p.pid)
		} else {
			log.Printf("terminated '%s' deployment, pid=%d\n", oldSpec.Name, p.pid)
		}

		updateDeploymentStatus(oldSpec.Name, Pending)

		newSpec, err := setYetisPortEnv(oldSpec)
		if err != nil {
			log.Printf("failed to set prot env for %s deployment: %s \n", newSpec.Name, err)
			return alive
		}

		_ = updateDeployment(newSpec, 0, "", false)
		pid, logPath, err := launchProcess(newSpec, false)
		if err != nil {
			log.Printf("Liveness failed to restart deployment '%s': %s\n", newSpec.Name, err)
		}
		_ = updateDeployment(newSpec, pid, logPath, true)
		thresholdMap.Delete(newSpec.Name)
		if newSpec.Proxy.Port > 0 {
			err := proxy.UpdatePortForwarding(newSpec.Proxy.Port, p.spec.LivenessProbe.Port(), newSpec.LivenessProbe.Port())
			if err != nil {
				log.Printf("Liveness restarted deployment, but failed to restart service: %s", err)
			} else {
				log.Printf("Liveness changed service of '%s' target port to %d\n", newSpec.Name, newSpec.LivenessProbe.Port())
			}
		}

		// wait for initial delay
		time.Sleep(newSpec.LivenessProbe.InitialDelayDuration())
		return alive
	}
	if tsh.SuccessCount >= dep.spec.LivenessProbe.SuccessThreshold {
		updateDeploymentStatus(dep.spec.Name, Running)
		if dep.status != Running { // if it wasn't already running
			// Status could be Pending after Failed. Threshold could be cleaned after Failed.
			// It will be triggered at the start of the process for example, but AlertRecovery checks if the process failed before.
			// not specifying the right condition will spam logs with "alert was not triggered"
			AlertRecovery(dep.spec.Name)
		}
	}
	return alive
}

var isPortOpenMock *bool

func isPortOpen(port int, dur time.Duration) bool {
	if isPortOpenMock != nil {
		return *isPortOpenMock
	}
	return common.DialPort(port, dur) == nil
}
