// Package orb is a thin wrapper around the OrbStack `orb` CLI. It does
// not call any private OrbStack APIs — every operation shells out and
// parses the documented JSON output of `orb list --format json` etc.
package orb

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Bin is the orb CLI binary name. Override in tests by pointing it at a
// fake script.
var Bin = "orb"

// IsInstalled reports whether `orb` is on PATH.
func IsInstalled() bool {
	_, err := exec.LookPath(Bin)
	return err == nil
}

// IsRunning reports whether the OrbStack daemon is up.
//
// `orb status` prints "Running" on stdout and exits 0 when up;
// non-zero otherwise.
func IsRunning() bool {
	out, err := exec.Command(Bin, "status").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(string(out)), "Running")
}

// IsUsable returns true when OrbStack is installed AND can be reached.
// If the daemon is down, IsUsable will attempt to start it once.
func IsUsable() bool {
	if !IsInstalled() {
		return false
	}
	if IsRunning() {
		return true
	}
	// Try to start; if start fails, treat as unusable (we never block on it).
	return EnsureRunning() == nil
}

// UnusableReason returns a short human-readable explanation when IsUsable
// would return false. Returns "" when usable.
func UnusableReason() string {
	if !IsInstalled() {
		return "orb CLI not on PATH"
	}
	if !IsRunning() {
		return "orb daemon not running and could not be started"
	}
	return ""
}

// EnsureRunning starts the OrbStack daemon if it isn't already.
func EnsureRunning() error {
	if IsRunning() {
		return nil
	}
	cmd := exec.Command(Bin, "start")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("orb start: %w", err)
	}
	return nil
}

// List returns every machine known to OrbStack.
func List() ([]Machine, error) {
	out, err := exec.Command(Bin, "list", "--format", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("orb list: %w", err)
	}
	var machines []Machine
	if err := json.Unmarshal(out, &machines); err != nil {
		return nil, fmt.Errorf("decode orb list: %w", err)
	}
	return machines, nil
}

// Find returns the machine with the given name, or (nil, nil) if it
// doesn't exist. Errors are reserved for other failures.
func Find(name string) (*Machine, error) {
	machines, err := List()
	if err != nil {
		return nil, err
	}
	for i := range machines {
		if machines[i].Name == name {
			return &machines[i], nil
		}
	}
	return nil, nil
}

// InfoFor returns the full info record for a single machine.
func InfoFor(name string) (*Info, error) {
	out, err := exec.Command(Bin, "info", name, "--format", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("orb info %s: %w", name, err)
	}
	var info Info
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("decode orb info: %w", err)
	}
	return &info, nil
}

// Create provisions a new machine.
func Create(opts CreateOptions) error {
	args := []string{"create"}
	if opts.Arch != "" {
		args = append(args, "-a", opts.Arch)
	}
	if opts.User != "" {
		args = append(args, "-u", opts.User)
	}
	if opts.Isolated {
		args = append(args, "--isolated")
	}
	for _, m := range opts.Mounts {
		args = append(args, "--mount", m.String())
	}

	var udPath string
	if len(opts.UserData) > 0 {
		f, err := os.CreateTemp("", "agent-helper-userdata-*.yaml")
		if err != nil {
			return fmt.Errorf("write user-data: %w", err)
		}
		udPath = f.Name()
		if _, err := f.Write(opts.UserData); err != nil {
			f.Close()
			os.Remove(udPath)
			return fmt.Errorf("write user-data: %w", err)
		}
		f.Close()
		defer os.Remove(udPath)
		args = append(args, "-c", udPath)
	}

	distro := opts.Distro
	if opts.Version != "" {
		distro = distro + ":" + opts.Version
	}
	args = append(args, distro, opts.Name)

	cmd := exec.Command(Bin, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("orb create %s: %w", opts.Name, err)
	}
	return nil
}

// Start a stopped machine.
func Start(name string) error {
	cmd := exec.Command(Bin, "start", name)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("orb start %s: %w", name, err)
	}
	return nil
}

// Stop a running machine.
func Stop(name string) error {
	cmd := exec.Command(Bin, "stop", name)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("orb stop %s: %w", name, err)
	}
	return nil
}

// Delete a machine, force-skipping the confirmation prompt.
func Delete(name string) error {
	cmd := exec.Command(Bin, "delete", "-f", name)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("orb delete %s: %w", name, err)
	}
	return nil
}
