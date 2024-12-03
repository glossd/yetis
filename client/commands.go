package client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"github.com/glossd/yetis/common/unix"
	"github.com/glossd/yetis/server"
	"os"
	"os/signal"
	"sigs.k8s.io/yaml"
	"syscall"
	"text/tabwriter"
	"time"
)

func init() {
	var baseURL = fmt.Sprintf("http://127.0.0.1:%d/deployments", server.YetisServerPort)
	fetch.SetBaseURL(baseURL)
}

func List() {
	printDeploymentTable()
}

func printDeploymentTable() (int, error) {
	views, err := fetch.Get[[]server.DeploymentView]("")
	if err != nil {
		fmt.Println(err)
		return 0, err
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tSTATUS\tPID\tRESTARTS\tAGE\tCOMMAND")
		for _, d := range views {
			fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%d\t%d\t%s\t%s", d.Name, d.Status, d.Pid, d.Restarts, d.Age, d.Command))
		}
		tw.Flush()
		return len(views), nil
	}
}

const upLine = "\033[A"

func ListWatch() {
	var returnToStart string
	go func() {
		// prevent 'signal: interrupt' message from being printed
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
		<-signalChan
		os.Exit(0)
	}()
	for {
		os.Stdout.WriteString(returnToStart)
		returnToStart = ""
		num, err := printDeploymentTable()
		if err != nil {
			return
		}
		returnToStart = upLine
		for i := 0; i < num; i++ {
			returnToStart += upLine
		}
		returnToStart += "\r"
		time.Sleep(time.Second)
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
	return common.IsPortOpen(server.YetisServerPort, time.Second)
}

func ShutdownServer() {
	pid, err := unix.GetPidByPort(server.YetisServerPort)
	if err != nil {
		fmt.Println("Couldn't get Yetis pid:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = unix.TerminateProcess(ctx, pid)
	if err != nil {
		fmt.Println("Failed to stop Yetis server", err)
	} else {
		fmt.Println("Yetis server stopped.")
	}
}
