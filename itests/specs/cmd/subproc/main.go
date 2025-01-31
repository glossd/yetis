package main

import (
	"net/http"
	"os/exec"
)

func main() {
	err := exec.Command("nc", "-lk", "27001").Start()
	if err != nil {
		panic(err)
	}
	err = http.ListenAndServe(":27000", nil)
	if err != nil {
		panic(err)
	}
}
