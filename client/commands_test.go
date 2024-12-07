package client

import (
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/server"
	"testing"
)

func TestInfo(t *testing.T) {
	go server.Run()
	defer server.Stop()

	res, err := fetch.Get[server.InfoResponse](baseHost + "/info")
	assert(t, err, nil)
	if res.Version == "" || res.NumberOfDeployments != 0 {
		t.Fatalf("Info() = %v", res)
	}
}

func assert[T comparable](t *testing.T, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("got %v, wanted %v", got, want)
	}
}
