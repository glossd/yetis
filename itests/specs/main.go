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
	port := os.Getenv("YETIS_PORT")
	if port == "" {
		panic("YETIS_PORT is not specified")
	}
	http.ListenAndServe(":"+port, nil)
}
