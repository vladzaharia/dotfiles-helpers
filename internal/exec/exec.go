package exec

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// FindBinary locates a binary in PATH and returns its absolute path.
func FindBinary(name string) (string, error) {
	return exec.LookPath(name)
}

// ValidateDeps checks that all named binaries are available in PATH.
// Returns a list of missing binaries.
func ValidateDeps(names ...string) []string {
	var missing []string
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	return missing
}

// Exec replaces the current process with the given binary (unix exec).
// This is used for dispatch — the helper process is replaced by the target tool.
func Exec(binary string, args []string) error {
	path, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("%s not found in PATH", binary)
	}
	argv := append([]string{binary}, args...)
	return syscall.Exec(path, argv, os.Environ())
}

// Run executes a command and returns its output.
func Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// RunPassthrough executes a command with stdin/stdout/stderr attached.
func RunPassthrough(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
