package server

import (
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"log"
	"sync"
	"time"
)

// name -> Threshold
var thresholdMap = sync.Map{}

type Threshold struct {
	SuccessCount int
	FailureCount int
}

var tickerMock *chan time.Time

// Non-blocking.
func runLivenessCheck(c common.DeploymentSpec, restartsLimit int) {
	time.AfterFunc(c.LivenessProbe.InitialDelayDuration(), func() {
		var ticker <-chan time.Time
		if tickerMock != nil {
			ticker = *tickerMock
		} else {
			ticker = time.NewTicker(c.LivenessProbe.PeriodDuration()).C
		}
		for ; true; <-ticker { // ;true; - to run instantly before the ticker
			stop := checkLiveness(c.Name, restartsLimit)
			if stop {
				return
			}
		}
	})
}

func checkLiveness(deploymentName string, restartsLimit int) bool {
	dep, ok := getDeployment(deploymentName)
	if !ok {
		// release go routine for GC
		return true
	}
	c := dep.spec

	var port = c.LivenessProbe.TcpSocket.Port
	// Remove 10 milliseconds for everything to process and wait for the new tick.
	portOpen := isPortOpen(port, c.LivenessProbe.PeriodDuration()-10*time.Millisecond)
	v, ok := thresholdMap.Load(c.Name)
	if !ok {
		v = Threshold{}
	}
	tsh := v.(Threshold)
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
			return true
		}
		if p.restarts >= restartsLimit {
			updateDeploymentStatus(c.Name, Failed)
			thresholdMap.Delete(c.Name)
			// linear backoff
			time.AfterFunc(time.Duration(restartsLimit/2)*time.Minute, func() {
				runLivenessCheck(c, restartsLimit*2)
			})
			return true
		}
		log.Printf("Restarting '%s' deployment, failureThreshold was reached\n", c.Name)
		updateDeploymentStatus(c.Name, Terminating)
		ctx, cancelCtx := context.WithTimeout(context.Background(), c.LivenessProbe.PeriodDuration())
		err := unix.TerminateProcess(ctx, p.pid)
		if err != nil {
			log.Printf("failed to terminate process, deployment=%s, pid=%d\n", c.Name, p.pid)
		} else {
			log.Printf("terminated '%s' deployment, pid=%d\n", c.Name, p.pid)
		}
		// todo instead of killing by port, terminate function should terminate all children as well.
		unix.KillByPort(c.LivenessProbe.TcpSocket.Port)

		cancelCtx()
		updateDeploymentStatus(c.Name, Pending)

		c, err := setDeploymentPortEnv(c)
		if err != nil {
			log.Printf("failed to set prot env for %s deployment: %s \n", c.Name, err)
			return false
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
		return false
	}
	if tsh.SuccessCount >= c.LivenessProbe.SuccessThreshold {
		updateDeploymentStatus(c.Name, Running)
	}
	return false
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
