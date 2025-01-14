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
	"path/filepath"
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

	logFilePath := filepath.Join(logdir, "yetis.log")
	file, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0750)
	if err != nil {
		fmt.Println("Failed to open log file at", logFilePath, err)
	}
	cmd := exec.Command("nohup", "yetis", "run")
	cmd.Stdout = file
	cmd.Stderr = file
	err = cmd.Start()
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
	fmt.Printf("Server: version=%s, deployments=%d, services=%d\n", get.Version, get.NumberOfDeployments, get.NumberOfServices)
}

func GetDeployments() {
	versionsWarning()
	printDeploymentTable()
}

func GetServices() {
	versionsWarning()
	printServiceTable()
}

func WatchGetDeployments() {
	watch(printDeploymentTable)
}

func WatchGetServices() {
	watch(printServiceTable)
}

func printDeploymentTable() (int, bool) {
	views, err := fetch.Get[[]server.DeploymentView]("/deployments")
	if err != nil {
		fmt.Println(err)
		return 0, false
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tSTATUS\tPID\tRESTARTS\tAGE\tCOMMAND\tPORT")
		for _, d := range views {
			fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%d\t%d\t%s\t%s\t%d", d.Name, d.Status, d.Pid, d.Restarts, d.Age, d.Command, d.LivenessPort))
		}
		tw.Flush()
		return len(views), true
	}
}

func printServiceTable() (int, bool) {
	views, err := fetch.Get[[]server.ServiceView]("/services")
	if err != nil {
		fmt.Println(err)
		return 0, false
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "SELECTORNAME\tPORT\tSTATUS\tDEPLOYMENTPORT\tPID")
		for _, s := range views {
			fmt.Fprintln(tw, fmt.Sprintf("%s\t%d\t%s\t%d\t%d", s.SelectorName, s.Port, s.Status, s.DeploymentPort, s.Pid))
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

func DescribeDeployment(name string) (server.GetResponse, error) {
	versionsWarning()
	r, err := GetDeployment(name)
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
	return r, nil
}

func GetDeployment(name string) (server.GetResponse, error) {
	return fetch.Get[server.GetResponse]("/deployments/" + name)
}

func DescribeService(selectorName string) {
	versionsWarning()
	r, err := fetch.Get[server.GetServiceResponse]("/services/" + selectorName)
	if err != nil {
		fmt.Println(err)
	} else {
		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprintf("PID: %d\n", r.Pid))
		buf.WriteString(fmt.Sprintf("Port: %d\n", r.Port))
		buf.WriteString(fmt.Sprintf("SelectorName: %s\n", r.SelectorName))
		buf.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
		buf.WriteString(fmt.Sprintf("DeploymentPort: %d\n", r.DeploymentPort))
		buf.WriteString(fmt.Sprintf("UpdatePort: %d\n", r.UpdatePort))
		fmt.Println(buf.String())
	}
}

func DeleteDeployment(name string) {
	versionsWarning()
	_, err := fetch.Delete[fetch.Empty]("/deployments/" + name)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Successfully deleted '%s' deployment\n", name)
	}
}

func DeleteService(selectorName string) {
	versionsWarning()
	_, err := fetch.Delete[fetch.Empty]("/services/" + selectorName)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Successfully deleted service for '%s'\n", selectorName)
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

func Restart(name string) error {
	_, err := fetch.Put[fetch.Empty]("/deployments/"+name+"/restart", nil)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Successfully restarted '%s' deployment\n", name)
	}
	return err
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
