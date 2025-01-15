package main

import (
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`OK`))
	})
	port := os.Getenv("APP_PORT")
	if port == "" {
		panic("APP_PORT is not specified")
	}
	http.ListenAndServe(":"+port, nil)
}
