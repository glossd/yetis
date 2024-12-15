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
	http.ListenAndServe(":"+os.Getenv("YETIS_PORT"), nil)
}
