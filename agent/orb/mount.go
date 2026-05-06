package orb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MountSpec is a host-to-VM mount definition for `orb create --mount`.
type MountSpec struct {
	Source string // host absolute path
	Dest   string // in-VM absolute path
}

// String returns the SOURCE[:DEST] form expected by `orb create --mount`.
func (m MountSpec) String() string {
	if m.Dest == "" || m.Dest == m.Source {
		return m.Source
	}
	return m.Source + ":" + m.Dest
}

// ExpandHome resolves a leading "~" in a path against $HOME.
func ExpandHome(p string) string {
	if p == "~" {
		h, _ := os.UserHomeDir()
		return h
	}
	if strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, p[2:])
	}
	return p
}

// SharedRoots returns the mount specs to apply when creating a shared
// agent VM, derived from configured host roots like ["~/Repos"].
//
// Convention: a host path "~/Repos" maps to "/host/Repos" inside the VM,
// so a CWD of "~/Repos/foo" is reachable at "/host/Repos/foo".
func SharedRoots(hostRoots []string) []MountSpec {
	specs := make([]MountSpec, 0, len(hostRoots))
	seen := map[string]struct{}{}
	for _, r := range hostRoots {
		src, err := filepath.Abs(ExpandHome(r))
		if err != nil || src == "" {
			continue
		}
		if _, dup := seen[src]; dup {
			continue
		}
		seen[src] = struct{}{}
		specs = append(specs, MountSpec{
			Source: src,
			Dest:   "/host/" + filepath.Base(src),
		})
	}
	return specs
}

// ResolveCWD maps a host CWD to its in-VM path under the given mount specs.
// Returns ok=false when the CWD is not under any mounted root.
func ResolveCWD(cwd string, specs []MountSpec) (vmPath string, ok bool) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", false
	}
	for _, m := range specs {
		rel, err := filepath.Rel(m.Source, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if rel == "." {
			return m.Dest, true
		}
		return filepath.Join(m.Dest, rel), true
	}
	return "", false
}

// StateMounts translates a Provider's StateDirs (e.g. ["~/.claude",
// "~/.claude.json"]) into MountSpecs that share the host paths into the
// VM at the equivalent /home/<user>/<rel> paths. Sources that do not
// exist on the host are silently skipped — fresh installs work without
// failing.
//
// OrbStack creates the VM's default user with the same name as the
// macOS user, so the path under ~ is identical between host and VM;
// only the $HOME prefix differs (/Users/<u> → /home/<u>).
func StateMounts(stateDirs []string) []MountSpec {
	hostHome, _ := os.UserHomeDir()
	username := currentUsername()
	if hostHome == "" || username == "" {
		return nil
	}
	vmHome := "/home/" + username
	specs := make([]MountSpec, 0, len(stateDirs))
	seen := map[string]struct{}{}
	for _, p := range stateDirs {
		if !strings.HasPrefix(p, "~/") {
			continue
		}
		rel := strings.TrimPrefix(p, "~/")
		if rel == "" {
			continue
		}
		src := filepath.Join(hostHome, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if _, dup := seen[src]; dup {
			continue
		}
		seen[src] = struct{}{}
		specs = append(specs, MountSpec{
			Source: src,
			Dest:   filepath.Join(vmHome, rel),
		})
	}
	return specs
}

func currentUsername() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	return ""
}

// MountSignature returns a short content-hash of the resolved mount set.
// Two mount sets with identical (source,dest) pairs hash identically
// regardless of order. Used to detect mount-config drift on existing
// VMs (which require recreate — `orb create --mount` is create-time only).
func MountSignature(mounts []MountSpec) string {
	parts := make([]string, len(mounts))
	for i, m := range mounts {
		parts[i] = m.Source + "→" + m.Dest
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])[:12]
}

// DedicatedMount returns the single mount spec for a per-project VM.
// Convention: $cwd → /work/<basename>.
func DedicatedMount(cwd string) (MountSpec, string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return MountSpec{}, "", fmt.Errorf("resolve cwd: %w", err)
	}
	dest := "/work/" + filepath.Base(abs)
	return MountSpec{Source: abs, Dest: dest}, dest, nil
}
