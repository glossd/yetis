package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const YetisServerPort = 54129
const YetisServerLogdirDefault = "/tmp"

func Start() {
	ysl := os.Getenv("YETIS_SERVER_LOGDIR")
	if ysl == "" {
		ysl = YetisServerLogdirDefault
	}
	if ysl != "stdout" {
		file, err := os.OpenFile(ysl+"/yetis.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Failed to yetis log file: %v", err)
		}
		defer file.Close()
		log.SetOutput(file)
	}
	log.SetFlags(log.LstdFlags) // adds time to the log

	mux := http.NewServeMux()

	mux.HandleFunc("GET /deployments", List)
	mux.HandleFunc("GET /deployments/{name}", Get)
	mux.HandleFunc("POST /deployments", Post)
	mux.HandleFunc("DELETE /deployments/{name}", Delete)

	runWithGracefulShutDown(mux)
}

// https://github.com/gin-gonic/examples/blob/master/graceful-shutdown/graceful-shutdown/server.go
func runWithGracefulShutDown(r *http.ServeMux) {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", YetisServerPort),
		Handler: r,
	}

	go func() {
		log.Printf("Starting server on %d\n", YetisServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Yetis server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %s", err)
	}

	log.Println("Yetis server exiting")
}
