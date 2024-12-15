package server

import (
	"testing"
)

func TestSortDeployments(t *testing.T) {
	res := []DeploymentView{{Name: "b"}, {Name: "a"}}
	sortDeployments(res)
	assert(t, res[0].Name, "a")
	assert(t, res[1].Name, "b")
}

func TestDeleteServiceWithDeleteDeployment(t *testing.T) {
	// todo
	//go Run()
	//defer Stop()
	//_, err := PostDeployment(common.DeploymentSpec{
	//	Name:          "hello",
	//	Cmd:           "nc -lk $YETIS_PORT",
	//	Logdir:        "stdout",
	//})
	//if err != nil {
	//	t.Fatal(err)
	//}
	//PostService(common.ServiceSpec{
	//	Port:     0,
	//	Selector: common.Selector{},
	//})
}

func TestServiceUpdateWhenDeploymentRestartOnRandomPort(t *testing.T) {

}
