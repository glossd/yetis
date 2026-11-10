package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/glossd/yetis/common"
	"github.com/tidwall/buntdb"
	"log"
	"path/filepath"
	"strings"
	"time"
)

var db *buntdb.DB

func initDB(dbDir string) func() error {
	var err error
	if dbDir == "" {
		log.Println("WARNING: No database path specified. Using in-memory setting.")
		db, err = buntdb.Open(":memory:")
	} else {
		// It will be created if it doesn't exist.
		db, err = buntdb.Open(filepath.Join(dbDir, common.YetisDbFileName))
	}
	if err != nil {
		log.Fatal("Failed to initialize bunt database", err)
	}
	return db.Close
}

type deployment struct {
	Pid       int                   `json:"pid"`
	LogPath   string                `json:"log_path"`
	Restarts  int                   `json:"restarts"`
	Status    ProcessStatus         `json:"status"`
	CreatedAt time.Time             `json:"created_at"`
	Spec      common.DeploymentSpec `json:"spec"`
}

func (d deployment) getPid() int {
	return d.Pid
}

func (d deployment) getPort() int {
	return d.Spec.LivenessProbe.Port()
}

func (d deployment) key() string {
	return depkey(d.Spec.Name)
}

func (d deployment) json() string {
	res, err := json.Marshal(d)
	if err != nil {
		log.Fatal(err)
	}
	return string(res)
}

func unmarshalDeployment(data string) (deployment, error) {
	var d deployment
	err := json.Unmarshal([]byte(data), &d)
	if err != nil {
		return deployment{}, err
	}
	return d, nil
}

type ProcessStatus int

const (
	Pending ProcessStatus = iota
	Running
	Failed
	Terminating
)

var processStatusMap = map[ProcessStatus]string{
	Pending:     "Pending",
	Running:     "Running",
	Failed:      "Failed",
	Terminating: "Terminating",
}

func (pc ProcessStatus) String() string {
	return processStatusMap[pc]
}

func saveDeployment(s common.DeploymentSpec, upsert bool) bool {
	dep := deployment{CreatedAt: time.Now(), Spec: s}

	err := db.Update(func(tx *buntdb.Tx) error {
		if !upsert {
			_, err := tx.Get(dep.key())
			if err == nil {
				return fmt.Errorf("strict insert failed: deployment '%s' already exists", dep.Spec.Name)
			} else if !errors.Is(err, buntdb.ErrNotFound) {
				return err
			}
		}
		_, _, err := tx.Set(dep.key(), dep.json(), nil)
		return err
	})

	if err != nil {
		if !strings.HasPrefix(err.Error(), "strict insert failed") {
			log.Println("saveDeployment failed with database error:", err)
		}
		return false
	}

	return true
}

func updateDeployment(s common.DeploymentSpec, pid int, logPath string, incRestarts bool) error {
	return setDeployment(s.Name, func(d deployment) deployment {
		d.Pid = pid
		d.LogPath = logPath
		if incRestarts {
			d.Restarts++
		}
		d.Spec = s
		return d
	})
}

func updateDeploymentStatus(name string, status ProcessStatus) {
	err := setDeployment(name, func(d deployment) deployment {
		d.Status = status
		return d
	})
	if err != nil {
		log.Printf("Failed to update deployment status '%s': %s\n", name, err)
	}
}

func setDeployment(name string, apply func(d deployment) deployment) error {
	err := db.Update(func(tx *buntdb.Tx) error {
		data, err := tx.Get(depkey(name))
		if err != nil {
			return err
		}
		d, err := unmarshalDeployment(data)
		if err != nil {
			return fmt.Errorf("failed to unmarshal deployment from buntdb: %v", err)
		}
		d = apply(d)
		_, _, err = tx.Set(d.key(), d.json(), nil)
		return err
	})
	if errors.Is(err, buntdb.ErrNotFound) {
		return fmt.Errorf("deployment %s doesn't exist", name)
	}
	return nil
}

func getDeployment(name string) (deployment, bool) {
	var d deployment
	err := db.View(func(tx *buntdb.Tx) error {
		data, err := tx.Get(depkey(name))
		if err != nil {
			return err
		}
		d, err = unmarshalDeployment(data)
		if err != nil {
			return err
		}
		return nil
	})
	return d, err == nil
}

func getDeploymentByRootName(rootName string) (deployment, int) {
	var dep deployment
	dep, found := getDeployment(rootName)
	if found {
		return dep, 1
	}

	var foundCount int
	// index could improve the speed
	err := rangeDeployments(func(name string, d deployment) {
		// todo the values could be unordered, so use counter and make sure its the highest
		if rootName == rootNameForRollingUpdate(name) {
			dep = d
			foundCount++
		}
	})
	if err != nil {
		log.Printf("Failed to get deployment by root name from buntdb: %v", err)
	}
	return dep, foundCount
}

func getDeploymentStatus(name string) (ProcessStatus, bool) {
	var dep deployment
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(depkey(name))
		if err != nil {
			return err
		}
		dep, err = unmarshalDeployment(val)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Pending, false
	}
	return dep.Status, true
}

func deleteDeployment(name string) error {
	return db.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete(depkey(name))
		return err
	})
}

func rangeDeployments(f func(namenum string, p deployment)) error {
	err := db.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend("", func(key, value string) bool {
			dep, err := unmarshalDeployment(value)
			if err != nil {
				log.Printf("Failed to unmarshal deployment from buntdb: %v ||| value=%s", err, value)
				return true // continue iteration
			}
			f(depnameFromKey(key), dep)
			return true // continue iteration
		})
		return err
	})
	return err
}

func getAllDeployments() ([]deployment, error) {
	var deps []deployment
	err := rangeDeployments(func(name string, p deployment) {
		deps = append(deps, p)
	})
	if err != nil {
		return nil, err
	}
	return deps, nil
}

func deploymentsNum() int {
	var num int
	db.View(func(tx *buntdb.Tx) error {
		num, _ = tx.Len()
		return nil
	})
	return num
}

func depkey(deploymentName string) string {
	return fmt.Sprintf("deployment:%s", deploymentName)
}

func depnameFromKey(deploymentKey string) string {
	length := len("deployment:")
	if len(deploymentKey) <= length {
		log.Println("Deployment key too short", deploymentKey)
		return deploymentKey
	}
	return deploymentKey[length:]
}
