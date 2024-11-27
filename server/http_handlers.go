package server

import (
	"fmt"
	"github.com/glossd/fetch"
	"github.com/glossd/yetis/common"
	"io"
	"log"
	"net/http"
	"text/tabwriter"
	"time"
)

type PostResponse struct {
	Success int
	Failure int
	Error   string
}

func Post(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		fetch.RespondError(w, 400, err)
		return
	}
	confs, err := fetch.Unmarshal[[]common.Config](string(b))
	if err != nil {
		fetch.RespondError(w, 400, fmt.Errorf("invalid configuration: %s", err))
		return
	}

	var errs []error
	for _, conf := range confs {
		err := applyConfig(conf)
		if err != nil {
			errs = append(errs, err)
		}
	}
	var errStr string
	for _, err := range errs {
		errStr = errStr + err.Error() + "\n"
	}

	err = fetch.Respond(w, &PostResponse{
		Success: len(confs) - len(errs),
		Failure: len(errs),
		Error:   errStr,
	})
	if err != nil {
		log.Println("Responding PostResponse error", err)
	}
}

func applyConfig(c common.Config) error {
	err := startDeployment(c)
	if err != nil {
		return err
	}
	runLivenessCheck(c, 2)
	return nil
}

func startDeployment(c common.Config) error {
	ok := saveDeployment(c, 0)
	if !ok {
		return fmt.Errorf("deployment '%s' already exists", c.Spec.Name)
	}
	pid, err := launchProcess(c)
	if err != nil {
		deleteDeployment(c.Spec.Name)
		return err
	}
	err = updateDeploymentPid(c.Spec.Name, pid)
	if err != nil {
		// For this to happen, delete must finish first before launching,
		// which is hard to imagine because start is asynchronous and delete is synchronous.
		log.Printf("Failed to update pid after launching process, pid=%d", pid)
		return err
	}
	return nil
}

func List(w http.ResponseWriter, r *http.Request) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	// todo, this table sucks.
	fmt.Fprintln(w, "NAME\tSTATUS\tPID\tRESTARTS\tAGE\tCOMMAND")
	rangeDeployments(func(name string, p deployment) {
		fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%d\t%d\t%s\t%s", name, p.status.String(), p.pid, p.restarts, ageSince(p.createdAt), p.config.Spec.Cmd))
	})
	tw.Flush()
}

func ageSince(t time.Time) string {
	age := time.Now().Sub(t)
	if age > 48*time.Hour {
		days := age.Hours() / 24
		return fmt.Sprintf("%.0fd", days)
	}
	if age > time.Hour {
		return fmt.Sprintf("%dh%dm", int(age.Hours()), int(age.Minutes())-(int(age.Hours())*60))
	}
	return fmt.Sprintf("%dm%ds", int(age.Minutes()), int(age.Seconds())-(int(age.Minutes())*60))
}

type GetResponse struct {
	Pid      int
	Restarts int
	Status   string
	Age      string
	Config   common.Config
}

func Get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		fetch.RespondError(w, 400, fmt.Errorf("name can't be empty"))
		return
	}
	p, ok := getDeployment(name)
	if !ok {
		fetch.RespondError(w, 400, fmt.Errorf("name '%s' doesn't exist", name))
		return
	}

	err := fetch.Respond(w, &GetResponse{
		Pid:      p.pid,
		Restarts: p.restarts,
		Status:   p.status.String(),
		Age:      ageSince(p.createdAt),
		Config:   p.config,
	})
	if err != nil {
		log.Println("Responding GetResponse error", err)
	}
}

func Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		fetch.RespondError(w, 400, fmt.Errorf(`name can't be empty`))
		return
	}

	d, ok := getDeployment(name)
	if !ok {
		fetch.RespondError(w, 404, fmt.Errorf(`'%s' doesn't exist'`, name))
		return
	}

	if d.pid != 0 {
		err := TerminateProcess(r.Context(), d.pid)
		if err != nil {
			fetch.RespondError(w, 500, err)
			return
		}
	}
	deleteDeployment(name)
}
