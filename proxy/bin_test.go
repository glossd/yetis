package proxy

import "testing"

func TestExec(t *testing.T) {
	pid, err := Exec(45678, 45679)
	if err != nil {
		t.Fatal(err)
	}
	if pid <= 0 {
		t.Fatalf("got pid: %d", pid)
	}
}
