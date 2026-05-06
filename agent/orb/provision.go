package orb

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// WaitCloudInit blocks until cloud-init finishes inside the named machine.
// Convenience wrapper for callers without a context.
func WaitCloudInit(machine string) error {
	return WaitCloudInitContext(context.Background(), machine)
}

// WaitCloudInitContext blocks until cloud-init finishes inside the named
// machine, returning ctx.Err() promptly if the context is cancelled. The
// retry loop sleeps 2s between attempts, so worst-case cancel latency is
// ~2s.
func WaitCloudInitContext(ctx context.Context, machine string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
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
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
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
	return writeMarker(machine, "/etc/agent-helper/bootstrap.version", version)
}

// ReadMountSignature returns the previously-stored mount signature, or "".
func ReadMountSignature(machine string) (string, error) {
	out, err := Capture(SSHOptions{
		Machine: machine,
		Argv:    []string{"cat", "/etc/agent-helper/mount.signature"},
	})
	if err != nil {
		return "", nil // missing → empty (not an error)
	}
	return strings.TrimSpace(out), nil
}

// WriteMountSignature stamps the resolved mount set's signature inside the VM.
func WriteMountSignature(machine, sig string) error {
	return writeMarker(machine, "/etc/agent-helper/mount.signature", sig)
}

func writeMarker(machine, path, content string) error {
	cmd := fmt.Sprintf("sudo install -d -m 0755 /etc/agent-helper && echo %s | sudo tee %s >/dev/null",
		shellQuote(content), shellQuote(path))
	_, err := Capture(SSHOptions{
		Machine: machine,
		Argv:    []string{"bash", "-lc", cmd},
	})
	return err
}
