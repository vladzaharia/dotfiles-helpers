package orb

import (
	"fmt"
	"strings"
	"time"
)

// WaitCloudInit blocks until cloud-init finishes inside the named machine.
// It runs `cloud-init status --wait` over SSH; that command exits 0 when
// the run is complete (success or already idle).
func WaitCloudInit(machine string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		out, err := Capture(SSHOptions{
			Machine: machine,
			Argv:    []string{"cloud-init", "status", "--wait"},
		})
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("cloud-init wait timed out: %v: %s", err, strings.TrimSpace(out))
		}
		time.Sleep(2 * time.Second)
	}
}

// ReadBootstrapVersion reads /etc/agent-helper/bootstrap.version from the VM.
// Returns "" with no error when the file does not exist (e.g. legacy VM
// created without agent-helper provisioning).
func ReadBootstrapVersion(machine string) (string, error) {
	out, err := Capture(SSHOptions{
		Machine: machine,
		Argv:    []string{"cat", "/etc/agent-helper/bootstrap.version"},
	})
	if err != nil {
		// File missing or VM missing → empty
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

// WriteBootstrapVersion records the current bootstrap version inside the VM.
func WriteBootstrapVersion(machine, version string) error {
	cmd := fmt.Sprintf("sudo install -d -m 0755 /etc/agent-helper && echo %s | sudo tee /etc/agent-helper/bootstrap.version >/dev/null",
		shellQuote(version))
	_, err := Capture(SSHOptions{
		Machine: machine,
		Argv:    []string{"bash", "-lc", cmd},
	})
	return err
}
