package orb

import (
	"fmt"
	"os"
	"path/filepath"
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
