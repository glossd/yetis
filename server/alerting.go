package server

import (
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"log"
)

var alertStore = common.Map[string, bool]{}

func AlertFail(name string) error {
	d, ok := getDeployment(name)
	if !ok {
		err := fmt.Errorf("deployment %s not found", name)
		log.Printf("AlertFail skipped: deployment %s not found\n", name)
		return err
	}
	_, loaded := alertStore.LoadOrStore(rootNameForRollingUpdate(name), true)
	if loaded {
		return fmt.Errorf("alert has already been sent")
	}

	info, err := fetch.Marshal(deploymentToInfo(d))
	if err != nil {
		log.Printf("AlertFail skipped: marshal: %s\n", err)
		return err
	}
	err = serverConfig.Alerting.Send(fmt.Sprintf("Deployment %s Failed", d.spec.Name), info)
	if err != nil {
		log.Printf("AlertFail skipped: send: %s", err)
		return err
	}
	return nil
}

func AlertRecovery(name string) error {
	_, loaded := alertStore.LoadAndDelete(rootNameForRollingUpdate(name))
	if !loaded {
		err := fmt.Errorf("alert not triggered for %s", name)
		log.Printf("AlertRecovery skipped: %s\n", err)
		return err
	}
	d, ok := getDeployment(name)
	if !ok {
		err := fmt.Errorf("deployment %s not found", name)
		log.Printf("AlertRecovery skipped: %s\n", err)
		return err
	}
	info, err := fetch.Marshal(deploymentToInfo(d))
	if err != nil {
		log.Printf("AlertRecovery skipped: marshal: %s\n", err)
		return err
	}
	err = serverConfig.Alerting.Send(fmt.Sprintf("Deployment %s Recovered", d.spec.Name), info)
	if err != nil {
		log.Printf("AlertRecovery skipped: send: %s", err)
		return err
	}
	return nil
}
