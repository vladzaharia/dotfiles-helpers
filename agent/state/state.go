// Package state persists agent-helper's per-session and per-VM bookkeeping
// to a small XDG-state directory tree:
//
//	~/.local/state/agent-helper/
//	├── sessions/<pid>.json   one per live invocation
//	└── vms/<name>.json       per-VM bootstrap + activity tracking
//
// Self-heal: every launch scans sessions/ and removes entries whose PID is
// no longer running. Per-VM owner_pids are filtered through the same liveness
// check before the VM is considered idle.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Dir returns the on-disk state root, honoring XDG_STATE_HOME.
func Dir() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "agent-helper")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "agent-helper")
}

func sessionsDir() string { return filepath.Join(Dir(), "sessions") }
func vmsDir() string      { return filepath.Join(Dir(), "vms") }

// Session is one live agent-helper invocation.
type Session struct {
	PID       int       `json:"pid"`
	VM        string    `json:"vm"`
	Agent     string    `json:"agent"`
	CWD       string    `json:"cwd"`
	StartedAt time.Time `json:"started_at"`
}

// VM is a per-machine record tracking bootstrap version and last activity.
type VM struct {
	Name             string    `json:"name"`
	Agent            string    `json:"agent"`
	BootstrapVersion string    `json:"bootstrap_version"`
	LastActive       time.Time `json:"last_active"`
	OwnerPIDs        []int     `json:"owner_pids"`
	// Packs is the list of cloud-init packs that were provisioned at
	// VM-create time. Used by dispatch to detect when a project needs
	// packs the shared VM doesn't have. Empty for VMs created before
	// pack tracking was added.
	Packs []string `json:"packs,omitempty"`
}

// OpenSession writes a sessions/<pid>.json marker.
func OpenSession(s Session) error {
	if s.PID == 0 {
		s.PID = os.Getpid()
	}
	if s.StartedAt.IsZero() {
		s.StartedAt = time.Now()
	}
	if err := os.MkdirAll(sessionsDir(), 0o755); err != nil {
		return err
	}
	return writeJSON(filepath.Join(sessionsDir(), strconv.Itoa(s.PID)+".json"), s)
}

// CloseSession removes the marker for the given PID.
func CloseSession(pid int) error {
	p := filepath.Join(sessionsDir(), strconv.Itoa(pid)+".json")
	err := os.Remove(p)
	if errIsNotExist(err) {
		return nil
	}
	return err
}

// Sessions returns every live session marker.
func Sessions() ([]Session, error) {
	entries, err := os.ReadDir(sessionsDir())
	if err != nil {
		if errIsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var s Session
		if err := readJSON(filepath.Join(sessionsDir(), e.Name()), &s); err == nil {
			out = append(out, s)
		}
	}
	return out, nil
}

// PruneStale removes session markers whose PIDs no longer exist.
// Returns the number of markers removed.
func PruneStale() int {
	sessions, _ := Sessions()
	n := 0
	for _, s := range sessions {
		if !pidAlive(s.PID) {
			if err := CloseSession(s.PID); err == nil {
				n++
			}
		}
	}
	return n
}

// LiveSessionsForVM returns sessions whose VM matches name and whose PID is alive.
func LiveSessionsForVM(name string) []Session {
	sessions, _ := Sessions()
	var out []Session
	for _, s := range sessions {
		if s.VM == name && pidAlive(s.PID) {
			out = append(out, s)
		}
	}
	return out
}

// RecordVM creates or updates the per-VM record. last_active is bumped to now.
func RecordVM(v VM) error {
	if err := os.MkdirAll(vmsDir(), 0o755); err != nil {
		return err
	}
	v.LastActive = time.Now()
	return writeJSON(filepath.Join(vmsDir(), v.Name+".json"), v)
}

// LoadVM reads a VM record, returning (nil, nil) if it doesn't exist.
func LoadVM(name string) (*VM, error) {
	var v VM
	err := readJSON(filepath.Join(vmsDir(), name+".json"), &v)
	if errIsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// VMs returns every recorded VM.
func VMs() ([]VM, error) {
	entries, err := os.ReadDir(vmsDir())
	if err != nil {
		if errIsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []VM
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var v VM
		if err := readJSON(filepath.Join(vmsDir(), e.Name()), &v); err == nil {
			out = append(out, v)
		}
	}
	return out, nil
}

// TouchVM bumps last_active to now and refreshes the persisted owner_pids
// to include the calling process. Idempotent — safe to call repeatedly
// during a session.
func TouchVM(name, agent, bootstrapVer string) error {
	v, err := LoadVM(name)
	if err != nil {
		return err
	}
	if v == nil {
		v = &VM{Name: name, Agent: agent, BootstrapVersion: bootstrapVer}
	}
	v.Agent = agent
	if bootstrapVer != "" {
		v.BootstrapVersion = bootstrapVer
	}
	v.OwnerPIDs = uniqueAlive(append(v.OwnerPIDs, os.Getpid()))
	return RecordVM(*v)
}

// ReleaseVM removes the calling pid from owner_pids; if it becomes empty,
// the record's last_active stays as the moment of release.
func ReleaseVM(name string) error {
	v, err := LoadVM(name)
	if err != nil || v == nil {
		return err
	}
	pid := os.Getpid()
	v.OwnerPIDs = filterPIDs(uniqueAlive(v.OwnerPIDs), pid)
	v.LastActive = time.Now()
	return RecordVM(*v)
}

// RecordVMPacks updates just the Packs field on a VM record. Called
// from the shared-VM runner after a fresh provision so dispatch can
// later compare requested packs against what's actually installed.
func RecordVMPacks(name string, packs []string) error {
	v, err := LoadVM(name)
	if err != nil {
		return err
	}
	if v == nil {
		v = &VM{Name: name}
	}
	v.Packs = append([]string(nil), packs...)
	return RecordVM(*v)
}

// ForgetVM deletes a VM record (call after `orb delete`).
func ForgetVM(name string) error {
	err := os.Remove(filepath.Join(vmsDir(), name+".json"))
	if errIsNotExist(err) {
		return nil
	}
	return err
}

// ----- helpers -----

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; signal 0 probes existence.
	return p.Signal(syscall.Signal(0)) == nil
}

func uniqueAlive(pids []int) []int {
	seen := map[int]bool{}
	out := pids[:0]
	for _, p := range pids {
		if seen[p] || !pidAlive(p) {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func filterPIDs(pids []int, drop int) []int {
	out := pids[:0]
	for _, p := range pids {
		if p != drop {
			out = append(out, p)
		}
	}
	return out
}

func writeJSON(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode %s: %w", path, err)
	}
	f.Close()
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(v)
}

func errIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
