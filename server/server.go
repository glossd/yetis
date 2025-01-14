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

const YetisServerPort = 54129

func Run() {
	log.SetFlags(log.LstdFlags) // adds time to the log

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

	mux.HandleFunc("GET /services", fetch.ToHandlerFunc(ListService))
	mux.HandleFunc("GET /services/{name}", fetch.ToHandlerFunc(GetService))
	mux.HandleFunc("POST /services", fetch.ToHandlerFunc(PostService))
	mux.HandleFunc("DELETE /services/{name}", fetch.ToHandlerFunc(DeleteService))
	mux.HandleFunc("PUT /services/{name}/port", fetch.ToHandlerFuncEmptyOut(UpdateServiceTargetPort))

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
	deleteServicesGracefully()

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

func deleteServicesGracefully() {
	serviceStore.Range(func(name string, value service) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := DeleteService(fetch.Request[fetch.Empty]{
			Context:    ctx,
			PathValues: map[string]string{"name": name},
		})
		if err == nil {
			log.Println("Deleted service for", name)
		} else {
			log.Printf("Failed to delete service for %s: %s\n", name, err)
		}
		return true
	})
}

type InfoResponse struct {
	Version             string
	NumberOfDeployments int
	NumberOfServices    int
}

func Info(_ fetch.Empty) (*InfoResponse, error) {
	return &InfoResponse{
		Version:             common.YetisVersion,
		NumberOfDeployments: deploymentsNum(),
		NumberOfServices:    servicesNum(),
	}, nil
}
