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
	"os/exec"
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

func StartBackground(logdir string) {
	if !unix.ExecutableExists("yetis") {
		fmt.Println("yetis is not installed")
	}

	logFilePath := logdir + "/yetis.log"
	err := exec.Command("nohup", "yetis", "run", ">>", logFilePath, "2>&1", "&").Start()
	if err != nil {
		fmt.Println("Failed to start Yetis:", err)
	}
	time.Sleep(25 * time.Millisecond)
	if !common.IsPortOpenUntil(server.YetisServerPort, 25*time.Millisecond, 10) {
		fmt.Println("Yetis hasn't started, check the log at", logFilePath)
	}
	fmt.Println("Yetis started successfully")
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
	preventSignalInterrupt()
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

func preventSignalInterrupt() {
	go func() {
		// prevent 'signal: interrupt' message from being printed on exit with Ctr^C
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
		<-signalChan
		os.Exit(0)
	}()
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
		buf.WriteString(fmt.Sprintf("Log Path: %s\n", r.LogPath))
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

func Logs(name string, stream bool) {
	r, err := fetch.Get[server.GetResponse]("/" + name)
	if err != nil {
		fmt.Println(err)
	} else {
		if stream {
			preventSignalInterrupt()
		}
		err := unix.Cat(r.LogPath, stream)
		if err != nil {
			fmt.Println("failed to print log file", err)
		}
	}
}

func IsServerRunning() bool {
	return common.IsPortOpen(server.YetisServerPort)
}

func ShutdownServer() {
	pid, err := unix.GetPidByPort(server.YetisServerPort)
	if err != nil {
		fmt.Println("Couldn't get Yetis pid:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	err = unix.TerminateProcess(ctx, pid)
	if err != nil {
		fmt.Println("Failed to stop Yetis server", err)
	} else {
		fmt.Println("Yetis server stopped.")
	}
}
