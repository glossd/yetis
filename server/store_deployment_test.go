package server

import (
	"github.com/tidwall/buntdb"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glossd/yetis/common"
)

// AI generated tests to make sure the migration from sync.Map to buntDB is compatible.

// resetDeploymentStore empties the global store so tests don't leak into each other.
func resetDeploymentStore(t *testing.T) {
	t.Helper()
	clear := func() {
		// Close the current in-memory DB to release locks
		if db != nil {
			err := db.Close()
			if err != nil {
				t.Errorf("failed to close database: %s", err)
			}
		}

		// Re-open a completely fresh, blank slate in-memory
		var err error
		db, err = buntdb.Open(":memory:")
		if err != nil {
			t.Fatal(err)
		}
	}
	clear()
	t.Cleanup(clear)
}

func spec(name string) common.DeploymentSpec {
	return common.DeploymentSpec{Name: name}
}

func TestProcessStatusString(t *testing.T) {
	tests := []struct {
		status ProcessStatus
		want   string
	}{
		{Pending, "Pending"},
		{Running, "Running"},
		{Failed, "Failed"},
		{Terminating, "Terminating"},
		{ProcessStatus(100), ""}, // unknown statuses map to the empty string
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("ProcessStatus(%d).String() = %q, want %q", int(tt.status), got, tt.want)
		}
	}
}

func TestSaveDeployment(t *testing.T) {
	t.Run("new deployment is saved", func(t *testing.T) {
		resetDeploymentStore(t)
		if !saveDeployment(spec("a"), false) {
			t.Fatal("expected save of a new deployment to succeed")
		}
		d, ok := getDeployment("a")
		if !ok {
			t.Fatal("deployment not found after save")
		}
		if d.Spec.Name != "a" {
			t.Errorf("Spec name = %q, want %q", d.Spec.Name, "a")
		}
		if d.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if d.Status != Pending {
			t.Errorf("initial Status = %v, want Pending", d.Status)
		}
	})

	t.Run("duplicate without upsert is rejected", func(t *testing.T) {
		resetDeploymentStore(t)
		saveDeployment(spec("a"), false)
		if saveDeployment(spec("a"), false) {
			t.Fatal("expected duplicate save without upsert to return false")
		}
		if n := deploymentsNum(); n != 1 {
			t.Errorf("deploymentsNum() = %d, want 1", n)
		}
	})

	t.Run("duplicate with upsert overwrites", func(t *testing.T) {
		resetDeploymentStore(t)
		saveDeployment(spec("a"), false)
		if err := updateDeployment(spec("a"), 42, "/tmp/a.log", true); err != nil {
			t.Fatal(err)
		}
		if !saveDeployment(spec("a"), true) {
			t.Fatal("expected upsert to return true")
		}
		d, _ := getDeployment("a")
		// upsert replaces the whole record, so runtime info is reset
		if d.Pid != 0 || d.LogPath != "" || d.Restarts != 0 {
			t.Errorf("upsert should reset runtime fields, got pid=%d LogPath=%q Restarts=%d", d.Pid, d.LogPath, d.Restarts)
		}
	})

	t.Run("concurrent saves without upsert: only one wins", func(t *testing.T) {
		resetDeploymentStore(t)
		const workers = 50
		var wins int32
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if saveDeployment(spec("race"), false) {
					atomic.AddInt32(&wins, 1)
				}
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Errorf("wins = %d, want exactly 1", wins)
		}
	})
}

func TestUpdateDeployment(t *testing.T) {
	t.Run("missing deployment returns error", func(t *testing.T) {
		resetDeploymentStore(t)
		if err := updateDeployment(spec("nope"), 1, "/log", false); err == nil {
			t.Fatal("expected error for non-existent deployment")
		}
		if _, ok := getDeployment("nope"); ok {
			t.Error("failed update must not create the deployment")
		}
	})

	t.Run("sets pid, log path and Spec", func(t *testing.T) {
		resetDeploymentStore(t)
		saveDeployment(spec("a"), false)
		if err := updateDeployment(spec("a"), 123, "/var/log/a.log", false); err != nil {
			t.Fatal(err)
		}
		d, _ := getDeployment("a")
		if d.Pid != 123 || d.getPid() != 123 {
			t.Errorf("pid = %d, want 123", d.Pid)
		}
		if d.LogPath != "/var/log/a.log" {
			t.Errorf("LogPath = %q", d.LogPath)
		}
		if d.Restarts != 0 {
			t.Errorf("Restarts = %d, want 0 when incRestarts=false", d.Restarts)
		}
	})

	t.Run("increments Restarts only when asked", func(t *testing.T) {
		resetDeploymentStore(t)
		saveDeployment(spec("a"), false)
		_ = updateDeployment(spec("a"), 1, "l", true)
		_ = updateDeployment(spec("a"), 2, "l", true)
		_ = updateDeployment(spec("a"), 3, "l", false)
		d, _ := getDeployment("a")
		if d.Restarts != 2 {
			t.Errorf("Restarts = %d, want 2", d.Restarts)
		}
		if d.Pid != 3 {
			t.Errorf("pid = %d, want 3", d.Pid)
		}
	})

	t.Run("preserves CreatedAt and Status", func(t *testing.T) {
		resetDeploymentStore(t)
		saveDeployment(spec("a"), false)
		updateDeploymentStatus("a", Running)
		before, _ := getDeployment("a")
		_ = updateDeployment(spec("a"), 9, "l", true)
		after, _ := getDeployment("a")
		if !after.CreatedAt.Equal(before.CreatedAt) {
			t.Error("CreatedAt changed on update")
		}
		if after.Status != Running {
			t.Errorf("Status = %v, want Running", after.Status)
		}
	})
}

func TestUpdateDeploymentStatus(t *testing.T) {
	t.Run("updates existing deployment", func(t *testing.T) {
		resetDeploymentStore(t)
		saveDeployment(spec("a"), false)
		for _, s := range []ProcessStatus{Running, Failed, Terminating} {
			updateDeploymentStatus("a", s)
			got, ok := getDeploymentStatus("a")
			if !ok || got != s {
				t.Errorf("Status = %v (ok=%v), want %v", got, ok, s)
			}
		}
	})

	t.Run("missing deployment is a no-op", func(t *testing.T) {
		resetDeploymentStore(t)
		updateDeploymentStatus("ghost", Running) // must not panic or create an entry
		if _, ok := getDeployment("ghost"); ok {
			t.Error("Status update must not create a deployment")
		}
	})
}

func TestGetDeploymentStatus(t *testing.T) {
	resetDeploymentStore(t)

	status, ok := getDeploymentStatus("missing")
	if ok {
		t.Error("expected ok=false for missing deployment")
	}
	if status != Pending {
		t.Errorf("Status for missing = %v, want Pending", status)
	}

	saveDeployment(spec("a"), false)
	updateDeploymentStatus("a", Failed)
	status, ok = getDeploymentStatus("a")
	if !ok || status != Failed {
		t.Errorf("got (%v, %v), want (Failed, true)", status, ok)
	}
}

func TestGetDeploymentByRootName(t *testing.T) {
	t.Run("exact match returns count 1", func(t *testing.T) {
		resetDeploymentStore(t)
		saveDeployment(spec("web"), false)
		d, n := getDeploymentByRootName("web")
		if n != 1 {
			t.Fatalf("count = %d, want 1", n)
		}
		if d.Spec.Name != "web" {
			t.Errorf("name = %q, want web", d.Spec.Name)
		}
	})

	t.Run("no match returns count 0", func(t *testing.T) {
		resetDeploymentStore(t)
		saveDeployment(spec("web"), false)
		_, n := getDeploymentByRootName("db")
		if n != 0 {
			t.Errorf("count = %d, want 0", n)
		}
	})

	t.Run("empty store returns count 0", func(t *testing.T) {
		resetDeploymentStore(t)
		_, n := getDeploymentByRootName("anything")
		if n != 0 {
			t.Errorf("count = %d, want 0", n)
		}
	})
}

func TestDeleteDeployment(t *testing.T) {
	resetDeploymentStore(t)
	saveDeployment(spec("a"), false)
	saveDeployment(spec("b"), false)

	deleteDeployment("a")
	if _, ok := getDeployment("a"); ok {
		t.Error("a should be deleted")
	}
	if _, ok := getDeployment("b"); !ok {
		t.Error("b should be untouched")
	}

	deleteDeployment("does-not-exist") // must not panic
	if n := deploymentsNum(); n != 1 {
		t.Errorf("deploymentsNum() = %d, want 1", n)
	}

	// name is free again after deletion
	if !saveDeployment(spec("a"), false) {
		t.Error("expected to be able to re-save a deleted deployment")
	}
}

func TestRangeDeployments(t *testing.T) {
	resetDeploymentStore(t)
	names := []string{"a", "b", "c"}
	for _, n := range names {
		saveDeployment(spec(n), false)
	}

	seen := map[string]bool{}
	rangeDeployments(func(name string, p deployment) {
		if p.Spec.Name != name {
			t.Errorf("key %q does not match Spec name %q", name, p.Spec.Name)
		}
		seen[name] = true
	})
	if len(seen) != len(names) {
		t.Errorf("visited %d deployments, want %d", len(seen), len(names))
	}
	for _, n := range names {
		if !seen[n] {
			t.Errorf("deployment %q was not visited", n)
		}
	}
}

func TestDeploymentsNum(t *testing.T) {
	resetDeploymentStore(t)
	if n := deploymentsNum(); n != 0 {
		t.Errorf("empty store: got %d, want 0", n)
	}
	saveDeployment(spec("a"), false)
	saveDeployment(spec("b"), false)
	if n := deploymentsNum(); n != 2 {
		t.Errorf("got %d, want 2", n)
	}
	deleteDeployment("a")
	if n := deploymentsNum(); n != 1 {
		t.Errorf("after delete: got %d, want 1", n)
	}
}

func TestConcurrentUpdatesAreSerialized(t *testing.T) {
	resetDeploymentStore(t)
	saveDeployment(spec("a"), false)

	const workers = 100
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = updateDeployment(spec("a"), 1, "l", true)
		}()
	}
	wg.Wait()

	d, _ := getDeployment("a")
	if d.Restarts != workers {
		t.Errorf("Restarts = %d, want %d (lost updates indicate a race)", d.Restarts, workers)
	}
}
