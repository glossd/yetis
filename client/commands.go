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

var baseHost = fmt.Sprintf("http://127.0.0.1:%d", server.YetisServerPort)

func init() {
	var baseURL = fmt.Sprintf(baseHost)
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
	time.Sleep(15 * time.Millisecond)
	if !common.IsPortOpenRetry(server.YetisServerPort, 10*time.Millisecond, 20) {
		fmt.Println("Yetis hasn't started, check the log at", logFilePath)
	}
	fmt.Println("Yetis started successfully")
}

func Info() {
	get, err := fetch.Get[server.InfoResponse]("/info")
	if err != nil {
		fmt.Println("Server hasn't responded", err)
	}
	fmt.Printf("Server: version=%s, deployments=%d\n", get.Version, get.NumberOfDeployments)
}

func GetDeployments() {
	versionsWarning()
	printDeploymentTable()
}

func WatchGetDeployments() {
	watch(printDeploymentTable)
}

func printDeploymentTable() (int, bool) {
	views, err := fetch.Get[[]server.DeploymentView]("/deployments")
	if err != nil {
		fmt.Println(err)
		return 0, false
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tSTATUS\tPID\tRESTARTS\tAGE\tCOMMAND")
		for _, d := range views {
			fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%d\t%d\t%s\t%s", d.Name, d.Status, d.Pid, d.Restarts, d.Age, d.Command))
		}
		tw.Flush()
		return len(views), true
	}
}

const upLine = "\033[A"

func watch(f func() (numberOfLines int, ok bool)) {
	var returnToStart string
	preventSignalInterrupt()
	for {
		os.Stdout.WriteString(returnToStart)
		returnToStart = ""
		numberOfLines, ok := f()
		if !ok {
			return
		}
		returnToStart = upLine
		for i := 0; i < numberOfLines; i++ {
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
	versionsWarning()
	r, err := fetch.Get[server.GetResponse]("/deployments/" + name)
	if err != nil {
		fmt.Println(err)
	} else {
		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprintf("PID: %d\n", r.Pid))
		buf.WriteString(fmt.Sprintf("Restarts: %d\n", r.Restarts))
		buf.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
		buf.WriteString(fmt.Sprintf("Age: %s\n", r.Age))
		buf.WriteString(fmt.Sprintf("Log Path: %s\n", r.LogPath))
		c, err := yaml.Marshal(r.Spec)
		if err != nil {
			panic("failed to marshal config" + err.Error())
		}
		buf.Write(c)
		fmt.Println(buf.String())
	}
}

func Delete(name string) {
	versionsWarning()
	_, err := fetch.Delete[fetch.Empty]("/deployments/" + name)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Successfully deleted '%s'\n", name)
	}
}

func Apply(path string) []error {
	versionsWarning()
	configs, err := common.ReadConfigs(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	var errs []error
	for _, config := range configs {
		switch config.Spec.Kind() {
		case common.Deployment:
			spec := config.Spec.(common.DeploymentSpec)
			_, err := fetch.Post[fetch.Empty]("/deployments", spec)
			if err != nil {
				errs = append(errs, err)
				fmt.Printf("Failure applying %s deployment: %s\n", spec.Name, err)
			} else {
				fmt.Printf("Successfully applied %s deployment\n", spec.Name)
			}
		case common.Service:
			spec := config.Spec.(common.ServiceSpec)
			_, err := fetch.Post[fetch.Empty]("/services", spec)
			if err != nil {
				errs = append(errs, err)
				fmt.Printf("Failure applying service for %s deployment: %s\n", spec.Selector.Name, err)
			} else {
				fmt.Printf("Successfully applied service for %s deployment\n", spec.Selector.Name)
			}
		}
	}
	return errs
}

func Logs(name string, stream bool) {
	r, err := fetch.Get[server.GetResponse]("/deployments/" + name)
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

func ShutdownServer(timeout time.Duration) {
	pid, err := unix.GetPidByPort(server.YetisServerPort)
	if err != nil {
		fmt.Println("Couldn't get Yetis pid:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err = unix.TerminateProcess(ctx, pid)
	if err != nil {
		fmt.Println("Failed to stop Yetis server", err)
	} else {
		fmt.Println("Yetis server stopped.")
	}
}

func versionsWarning() {
	get, err := fetch.Get[server.InfoResponse]("/info")
	if err == nil {
		if get.Version != common.YetisVersion {
			fmt.Printf("Warning: yetis version mismatch, client=%s, server=%s\n", common.YetisVersion, get.Version)
		}
	}
}
