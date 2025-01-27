package server

import (
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ports from 0-1023 are system or well-known ports.
// ports from 1024-49151 are user or registered.
// ports from 49152-65535 are called dynamic.

const YetisServerPort = 34711

func Run() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds) // adds time to the log

	mux := http.NewServeMux()

	fetch.SetHandlerConfig(fetch.HandlerConfig{
		ErrorHook: func(err error) {
			log.Println("fetch.Handler error", err)
		},
	})
	mux.HandleFunc("GET /info", fetch.ToHandlerFunc(Info))

	mux.HandleFunc("GET /deployments", fetch.ToHandlerFuncEmptyIn(ListDeployment))
	mux.HandleFunc("GET /deployments/{name}", fetch.ToHandlerFunc(GetDeployment))
	mux.HandleFunc("POST /deployments", fetch.ToHandlerFuncEmptyOut(PostDeployment))
	mux.HandleFunc("DELETE /deployments/{name}", fetch.ToHandlerFuncEmptyOut(DeleteDeployment))
	mux.HandleFunc("PUT /deployments/{name}/restart", fetch.ToHandlerFuncEmptyOut(RestartDeployment))

	runWithGracefulShutDown(mux)
}

var quit = make(chan os.Signal)
var finished = make(chan bool)

// https://github.com/gin-gonic/examples/blob/master/graceful-shutdown/graceful-shutdown/server.go
func runWithGracefulShutDown(r *http.ServeMux) {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", YetisServerPort),
		Handler: r,
	}

	go func() {
		log.Printf("Starting server %s on port %d\n", common.YetisVersion, YetisServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to listen: %s\n", err)
		}
	}()

	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("Shutting down Yetis server %s...\n", common.YetisVersion)

	deleteDeploymentsGracefully()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %s", err)
	}

	log.Println("Yetis server exiting")
	finished <- true
}

func Stop() {
	quit <- syscall.SIGTERM
	<-finished
}

func deleteDeploymentsGracefully() {
	rangeDeployments(func(name string, p deployment) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := DeleteDeployment(fetch.Request[fetch.Empty]{
			Context:    ctx,
			PathValues: map[string]string{"name": name},
		})
		if err == nil {
			log.Println("Deleted deployment", name)
		} else {
			log.Printf("Failed to delete %s deployment: %s\n", name, err)
		}
	})
}

type InfoResponse struct {
	Version             string
	NumberOfDeployments int
}

func Info(_ fetch.Empty) (*InfoResponse, error) {
	return &InfoResponse{
		Version:             common.YetisVersion,
		NumberOfDeployments: deploymentsNum(),
	}, nil
}
