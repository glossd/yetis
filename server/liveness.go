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
	runLivenessCheck(c, defaultRestartLimit, nil)
}

// Non-blocking.
func runLivenessCheck(c common.DeploymentSpec, restartsLimit int, stop chan bool) {
	if stop == nil {
		stop = make(chan bool)
	}
	livenessMap.Store(c.Name, stop)
	time.AfterFunc(c.LivenessProbe.InitialDelayDuration(), func() {
		var ticker = time.NewTicker(c.LivenessProbe.PeriodDuration()).C

		cleanUp := func() {
			livenessMap.Delete(c.Name)
			thresholdMap.Delete(c.Name)
		}
		// check instantly
		if t := heartbeat(c.Name, restartsLimit); t == dead {
			cleanUp()
			return
		}
		for {
			select {
			case <-stop:
				cleanUp()
				return
			case <-ticker:
				switch heartbeat(c.Name, restartsLimit) {
				case dead:
					cleanUp()
					return
				case tryAgain:
					time.AfterFunc(time.Duration(restartsLimit/2)*time.Minute, func() {
						runLivenessCheck(c, restartsLimit*2, stop)
					})
					return
				}
			}
		}
	})
}

func deleteLivenessCheck(name string) bool {
	v, ok := livenessMap.Load(name)
	if ok {
		v <- true
		close(v)
		return true
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
	c := dep.spec

	var port = c.LivenessProbe.TcpSocket.Port
	// Remove 10 milliseconds for everything to process and wait for the new tick.
	portOpen := isPortOpen(port, c.LivenessProbe.PeriodDuration()-10*time.Millisecond)
	tsh, ok := thresholdMap.Load(c.Name)
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
	thresholdMap.Store(c.Name, tsh)

	if tsh.FailureCount >= c.LivenessProbe.FailureThreshold {
		p, ok := getDeployment(c.Name)
		if !ok {
			log.Printf("Deployment '%s' reached failure threshold, but it doesn't exist\n", c.Name)
			return dead
		}
		if p.restarts >= restartsLimit {
			updateDeploymentStatus(c.Name, Failed)
			thresholdMap.Delete(c.Name)
			return tryAgain
		}
		log.Printf("Restarting '%s' deployment, failureThreshold was reached\n", c.Name)
		updateDeploymentStatus(c.Name, Terminating)
		ctx, cancelCtx := context.WithTimeout(context.Background(), c.LivenessProbe.PeriodDuration())
		err := terminateProcess(ctx, p)
		if err != nil {
			log.Printf("failed to terminate process, deployment=%s, pid=%d\n", c.Name, p.pid)
		} else {
			log.Printf("terminated '%s' deployment, pid=%d\n", c.Name, p.pid)
		}

		cancelCtx()
		updateDeploymentStatus(c.Name, Pending)

		c, err := setDeploymentPortEnv(c)
		if err != nil {
			log.Printf("failed to set prot env for %s deployment: %s \n", c.Name, err)
			return alive
		}

		updateDeployment(c, 0, "", false)
		pid, logPath, err := launchProcess(c)
		if err != nil {
			log.Printf("Liveness failed to restart deployment '%s': %s\n", c.Name, err)
		}
		updateDeployment(c, pid, logPath, true)
		thresholdMap.Delete(c.Name)
		err = updateServicePointingToNewPort(c)
		if err != nil {
			log.Printf("Liveness restarted deployment, but failed to restart service: %s", err)
		}
		// wait for initial delay
		time.Sleep(c.LivenessProbe.InitialDelayDuration())
		return alive
	}
	if tsh.SuccessCount >= c.LivenessProbe.SuccessThreshold {
		updateDeploymentStatus(c.Name, Running)
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

func updateServicePointingToNewPort(s common.DeploymentSpec) error {
	_, err := RestartService(fetch.Request[fetch.Empty]{Context: context.Background(), PathValues: map[string]string{"name": s.Name}})
	if err != nil {
		if ferr, ok := err.(*fetch.Error); ok && ferr.Status == 404 {
			return nil
		} else {
			return fmt.Errorf("failed to restart service for '%s' deployment: %s", s.Name, err)
		}
	}

	return nil
}
