package client

import (
	"bytes"
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

type Settings struct {
	Alerting
}

type Alerting interface {
}

type MailAlerting struct {
	Host     string
	From     string
	Username string
	Password string
}

func StartBackground(pathToConfig string) {
	if !unix.ExecutableExists("yetis") {
		fmt.Println("yetis is not installed")
	}

	c := common.YetisConfig{}.WithDefaults()
	if pathToConfig != "" {
		c = common.ReadServerConfig(pathToConfig)
	}

	logFilePath := filepath.Join(c.Logdir, "yetis.log")
	file, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0750)
	if err != nil {
		fmt.Println("Failed to open log file at", logFilePath, err)
	}
	cmd := exec.Command("nohup", "yetis", "run", "-f", pathToConfig)
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
	views, err := fetch.Get[[]server.DeploymentInfo]("/deployments")
	if err != nil {
		fmt.Println(err)
		return 0, false
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tSTATUS\tPID\tRESTARTS\tAGE\tCOMMAND\tPORT")
		for _, d := range views {
			fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%d\t%d\t%s\t%s\t%s", d.Name, d.Status, d.Pid, d.Restarts, d.Age, d.Command, d.PortInfo))
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

func DescribeDeployment(name string) {
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
}

func GetDeployment(name string) (server.DeploymentFullInfo, error) {
	return fetch.Get[server.DeploymentFullInfo]("/deployments/" + name)
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
		}
	}
	return errs
}

func Logs(name string, stream bool) {
	r, err := fetch.Get[server.DeploymentFullInfo]("/deployments/" + name)
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
	// todo it might take a while, need to have a status
	fmt.Println("Restarting deployment...")
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

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Couldn't find Yetis process %d: %s\n", pid, err)
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		fmt.Printf("Failed to terminate %d server: %s\n", pid, err)
	}
	after := time.After(timeout)
loop:
	for {
		select {
		case <-after:
			if !IsServerRunning() {
				break loop
			}
			err := process.Signal(syscall.SIGINT)
			if err != nil {
				fmt.Printf("Failed to terminate %d server rapidly: %s\n", pid, err)
			}
			time.Sleep(50 * time.Millisecond) // let server kill all deployments
			err = process.Kill()
			if err != nil {
				fmt.Printf("Failed to kill %d server rapidly: %s\n", pid, err)
			}
			if common.IsPortOpenRetry(server.YetisServerPort, 30*time.Millisecond, 20) {
				fmt.Println("Failed to kill Yetis server")
			} else {
				fmt.Println("Yetis server killed.")
			}
			return
		default:
			if IsServerRunning() {
				continue loop
			} else {
				break loop
			}
		}
	}
	fmt.Println("Yetis server stopped.")
}

func versionsWarning() {
	get, err := fetch.Get[server.InfoResponse]("/info")
	if err == nil {
		if get.Version != common.YetisVersion {
			fmt.Printf("Warning: yetis version mismatch, client=%s, server=%s\n", common.YetisVersion, get.Version)
		}
	}
}
