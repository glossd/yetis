package server

import (
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
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

	var port = dep.spec.LivenessProbe.TcpSocket.Port
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
		spec := p.spec
		if !ok {
			log.Printf("Deployment '%s' reached failure threshold, but it doesn't exist\n", spec.Name)
			return dead
		}
		if p.restarts >= restartsLimit {
			updateDeploymentStatus(spec.Name, Failed)
			thresholdMap.Delete(spec.Name)
			return tryAgain
		}
		log.Printf("Restarting '%s' deployment, failureThreshold was reached\n", spec.Name)
		updateDeploymentStatus(spec.Name, Terminating)
		ctx, cancelCtx := context.WithTimeout(context.Background(), spec.LivenessProbe.PeriodDuration())
		defer cancelCtx()
		err := terminateProcess(ctx, p)
		if err != nil {
			log.Printf("failed to terminate process, deployment=%s, pid=%d\n", spec.Name, p.pid)
		} else {
			log.Printf("terminated '%s' deployment, pid=%d\n", spec.Name, p.pid)
		}

		updateDeploymentStatus(spec.Name, Pending)

		spec, err = setYetisPortEnv(spec)
		if err != nil {
			log.Printf("failed to set prot env for %s deployment: %s \n", spec.Name, err)
			return alive
		}

		_ = updateDeployment(spec, 0, "", false)
		pid, logPath, err := launchProcess(spec)
		if err != nil {
			log.Printf("Liveness failed to restart deployment '%s': %s\n", spec.Name, err)
		}
		_ = updateDeployment(spec, pid, logPath, true)
		thresholdMap.Delete(spec.Name)
		ok, err = updateServiceTargetPortIfExists(ctx, spec)
		if err != nil {
			log.Printf("Liveness restarted deployment, but failed to restart service: %s", err)
		} else if ok {
			log.Printf("Liveness changed service of '%s' target port to %d\n", spec.Name, spec.YetisPort())
		}
		// wait for initial delay
		time.Sleep(spec.LivenessProbe.InitialDelayDuration())
		return alive
	}
	if tsh.SuccessCount >= dep.spec.LivenessProbe.SuccessThreshold {
		updateDeploymentStatus(dep.spec.Name, Running)
	}
	return alive
}

var isPortOpenMock *bool

func isPortOpen(port int, dur time.Duration) bool {
	if isPortOpenMock != nil {
		return *isPortOpenMock
	}
	return common.IsPortOpenTimeout(port, dur)
}

func updateServiceTargetPortIfExists(ctx context.Context, s common.DeploymentSpec) (bool, error) {
	err := UpdateServiceTargetPort(fetch.Request[int]{Context: ctx, Body: s.YetisPort()}.WithPathValue("name", rootNameForRollingUpdate(s.Name)))
	if err != nil {
		if ferr, ok := err.(*fetch.Error); ok && ferr.Status == 404 {
			return false, nil
		} else {
			return false, fmt.Errorf("failed to restart service for '%s' deployment: %s", s.Name, err)
		}
	}

	return true, nil
}
