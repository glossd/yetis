package server

import (
	"github.com/glossd/yetis/common"
	"strings"
	"testing"
)

func TestAlertRecovery_WithoutAlertFail(t *testing.T) {
	err := AlertRecovery("hello")
	assert(t, true, strings.HasPrefix(err.Error(), "alert not triggered for"))
}

func TestAlertFailAndRecovery(t *testing.T) {
	saveDeployment(common.DeploymentSpec{Name: "hello"}, false)
	err := AlertFail("hello")
	assert(t, err, nil)
	err = AlertRecovery("hello")
	assert(t, err, nil)
	err = AlertRecovery("hello")
	assert(t, true, strings.HasPrefix(err.Error(), "alert not triggered"))
}

func TestAlertFail_Double(t *testing.T) {
	saveDeployment(common.DeploymentSpec{Name: "hello"}, false)
	err := AlertFail("hello")
	assert(t, err, nil)
	err = AlertFail("hello")
	assert(t, err.Error(), "alert has already been sent")
}
