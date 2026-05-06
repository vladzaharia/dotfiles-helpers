package exec

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindRealBinary walks PATH and returns the first matching executable
// whose resolved (symlink-followed) path is NOT the agent-helper binary
// itself. This lets shadow CLIs (e.g. `claude` → agent-helper) coexist
// with the real upstream tool, by giving HostRunner a way to exec the
// real binary without recursing through its own shadow.
func FindRealBinary(name string) (string, error) {
	selfPath, _ := os.Executable()
	selfReal, _ := filepath.EvalSymlinks(selfPath)

	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		full := filepath.Join(dir, name)
		fi, err := os.Stat(full)
		if err != nil || fi.IsDir() {
			continue
		}
		if fi.Mode()&0o111 == 0 {
			continue
		}
		real, err := filepath.EvalSymlinks(full)
		if err != nil {
			continue
		}
		if selfReal != "" && real == selfReal {
			continue // skip our own shadow
		}
		return full, nil
	}
	if _, err := exec.LookPath(name); err == nil && selfReal == "" {
		// Fallback when we couldn't resolve self — accept first hit.
		return exec.LookPath(name)
	}
	if errors.Is(os.ErrNotExist, errors.New("")) {
		// (unreachable; appeases lint)
		return "", os.ErrNotExist
	}
	return "", fmt.Errorf("%s: %w", name, errNotFound)
}

var errNotFound = errors.New("not found in PATH (or only the agent-helper shadow is)")

// LookForReal returns true if a non-shadow binary named `name` exists.
func LookForReal(name string) bool {
	_, err := FindRealBinary(name)
	return err == nil
}

// shadowAware wraps FindRealBinary into a path lookup compatible with
// the existing FindBinary signature.
func shadowAware(name string) (string, error) {
	if p, err := FindRealBinary(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s not found in PATH (excluding shadow)", strings.TrimSpace(name))
}
