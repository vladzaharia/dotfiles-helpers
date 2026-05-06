package orb

import (
	"fmt"
	"strings"
)

// Reseed runs the given idempotent runcmd lines inside the VM and
// writes the new bootstrap version. Called when the embedded
// BootstrapPlan.Version differs from /etc/agent-helper/bootstrap.version
// inside the VM.
//
// All runcmd statements are concatenated into a single bash -lc invocation
// with `set -e` so a failed step aborts the whole reseed.
func Reseed(machine string, runcmd []string, version string) error {
	if len(runcmd) == 0 {
		return nil
	}
	script := "set -e\n" + strings.Join(runcmd, "\n") + "\n"
	if _, err := Capture(SSHOptions{
		Machine: machine,
		User:    "root",
		Argv:    []string{"bash", "-lc", script},
	}); err != nil {
		return fmt.Errorf("reseed: %w", err)
	}
	return WriteBootstrapVersion(machine, version)
}
