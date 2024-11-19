package client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/server"
	"sigs.k8s.io/yaml"
	"time"
)

func init() {
	var baseURL = fmt.Sprintf("http://127.0.0.1:%d/deployments", server.YetisServerPort)
	fetch.SetBaseURL(baseURL)
}

func List() {
	res, err := fetch.Get[string]("")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(res)
	}
}

func Describe(name string) {
	r, err := fetch.Get[server.GetResponse]("/" + name)
	if err != nil {
		fmt.Println(err)
	} else {
		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprintf("PID: %d\n", r.Pid))
		buf.WriteString(fmt.Sprintf("Restarts: %d\n", r.Restarts))
		buf.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
		buf.WriteString(fmt.Sprintf("Age: %s\n", r.Age))
		c, err := yaml.Marshal(r.Config)
		if err != nil {
			panic("failed to marshal config" + err.Error())
		}
		buf.Write(c)
		fmt.Println(buf.String())
	}
}

func Delete(name string) {
	_, err := fetch.Delete[fetch.ResponseEmpty]("/" + name)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Successfully deleted '%s'\n", name)
	}
}

func Apply(path string) {
	configs, err := common.ReadConfigs(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	res, err := fetch.Post[server.PostResponse]("", configs)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(fmt.Sprintf("Successfully applied %d out of %d", res.Success, res.Success+res.Failure))
		if res.Error != "" {
			fmt.Println(fmt.Sprintf("Errors: %s", res.Error))
		}
	}
}

func IsServerRunning() bool {
	return server.IsPortOpen(server.YetisServerPort, time.Second)
}

func ShutdownServer() {
	pid, err := server.GetPidByPort(server.YetisServerPort)
	if err != nil {
		fmt.Println("Couldn't get Yetis pid:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = server.TerminateProcess(ctx, pid)
	if err != nil {
		fmt.Println("Failed to stop Yetis server", err)
	} else {
		fmt.Println("Yetis server stopped.")
	}
}
