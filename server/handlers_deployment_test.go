package server

import (
	"testing"
)

func TestSortDeployments(t *testing.T) {
	res := []DeploymentInfo{{Name: "b"}, {Name: "a"}}
	sortDeployments(res)
	assert(t, res[0].Name, "a")
	assert(t, res[1].Name, "b")
}

func TestUpgradeNameForRollingUpdate(t *testing.T) {
	type testCase struct {
		I string
		O string
	}
	var cases = []testCase{
		{I: "hello", O: "hello-1"},
		{I: "hello--1", O: "hello--2"},
		{I: "hello-2", O: "hello-3"},
		{I: "hello-12", O: "hello-13"},
		{I: "hello-1-sec", O: "hello-1-sec-1"},
	}
	for _, c := range cases {
		got := upgradeNameForRollingUpdate(c.I)
		if c.O != got {
			t.Errorf("expected %s, got %s", c.O, got)
		}
	}
}

func TestRootNameForRollingUpdate(t *testing.T) {
	type testCase struct {
		I string
		O string
	}
	var cases = []testCase{
		{I: "hello-1", O: "hello"},
		{I: "hello", O: "hello"},
		{I: "hello-12", O: "hello"},
		{I: "hello-1-sec-1", O: "hello-1-sec"},
	}
	for _, c := range cases {
		got := rootNameForRollingUpdate(c.I)
		if c.O != got {
			t.Errorf("expected %s, got %s", c.O, got)
		}
	}
}
