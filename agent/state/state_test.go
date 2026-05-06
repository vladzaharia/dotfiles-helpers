package state

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempState(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if got := Dir(); got != filepath.Join(dir, "agent-helper") {
		t.Fatalf("Dir() = %q", got)
	}
}

func TestSessionLifecycle(t *testing.T) {
	withTempState(t)
	s := Session{PID: os.Getpid(), VM: "claude", Agent: "claude", CWD: "/tmp/x"}
	if err := OpenSession(s); err != nil {
		t.Fatal(err)
	}
	live, _ := Sessions()
	if len(live) != 1 {
		t.Fatalf("expected 1 session, got %d", len(live))
	}
	if err := CloseSession(s.PID); err != nil {
		t.Fatal(err)
	}
	live, _ = Sessions()
	if len(live) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(live))
	}
}

func TestPruneStaleRemovesDeadPID(t *testing.T) {
	withTempState(t)
	deadPID := 99999
	if err := OpenSession(Session{PID: deadPID, VM: "claude"}); err != nil {
		t.Fatal(err)
	}
	if n := PruneStale(); n != 1 {
		t.Fatalf("expected 1 prune, got %d", n)
	}
}

func TestVMTouchRelease(t *testing.T) {
	withTempState(t)
	if err := TouchVM("claude", "claude", "v1"); err != nil {
		t.Fatal(err)
	}
	v, _ := LoadVM("claude")
	if v == nil || v.BootstrapVersion != "v1" || len(v.OwnerPIDs) != 1 {
		t.Fatalf("unexpected vm record: %+v", v)
	}
	if err := ReleaseVM("claude"); err != nil {
		t.Fatal(err)
	}
	v, _ = LoadVM("claude")
	if len(v.OwnerPIDs) != 0 {
		t.Fatalf("expected 0 owner pids after release, got %d", len(v.OwnerPIDs))
	}
}
