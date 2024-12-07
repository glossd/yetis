package server

import (
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const YetisServerPort = 54129

func Run() {
	log.SetFlags(log.LstdFlags) // adds time to the log

	mux := http.NewServeMux()

	fetch.SetDefaultHandlerConfig(fetch.HandlerConfig{
		ErrorHook: func(err error) {
			log.Println("fetch.Handler error", err)
		},
	})
	mux.HandleFunc("GET /info", fetch.ToHandlerFunc(Info))
	mux.HandleFunc("GET /deployments", fetch.ToHandlerFunc(List))
	mux.HandleFunc("GET /deployments/{name}", fetch.ToHandlerFunc(Get))
	mux.HandleFunc("POST /deployments", fetch.ToHandlerFunc(Post))
	mux.HandleFunc("DELETE /deployments/{name}", fetch.ToHandlerFunc(Delete))

	runWithGracefulShutDown(mux)
}

var quit = make(chan os.Signal)

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

	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Yetis server...")

	deleteDeploymentsGracefully()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %s", err)
	}

	log.Println("Yetis server exiting")
}

func Stop() {
	quit <- syscall.SIGTERM
}

func deleteDeploymentsGracefully() {
	rangeDeployments(func(name string, p deployment) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := Delete(DeleteRequest{
			Ctx:  ctx,
			Name: name,
		})
		if err == nil {
			log.Println("Deleted", name)
		} else {
			log.Printf("Failed to delete %s: %s\n", name, err)
		}
	})
}
