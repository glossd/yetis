package common

import "testing"

func TestGetFreePort(t *testing.T) {
	p, err := GetFreePort()
	assert(t, err, nil)
	if p == 0 {
		t.Errorf("port 0")
	}
}
