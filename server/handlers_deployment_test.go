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
