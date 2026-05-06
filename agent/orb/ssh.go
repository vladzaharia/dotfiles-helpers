package orb

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SSHOptions controls how `ssh <user>@<machine>@orb` is invoked.
type SSHOptions struct {
	Machine     string   // machine name (required)
	User        string   // empty = OrbStack default user
	Interactive bool     // pass -t for a TTY
	Workdir     string   // optional cd target inside the VM (executed via bash -lc)
	Argv        []string // command + args; if empty, opens an interactive shell
	Env         []string // extra env vars to set: KEY=VAL
}

const (
	configRel  = ".orbstack/ssh/config"
	muxDirName = ".orbstack/run"
)

// sshArgs assembles the full argv for /usr/bin/ssh.
func sshArgs(opts SSHOptions) ([]string, error) {
	if opts.Machine == "" {
		return nil, fmt.Errorf("ssh: machine name required")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cfg := filepath.Join(home, configRel)
	muxDir := filepath.Join(home, muxDirName)
	muxPath := filepath.Join(muxDir, "agent-helper-%C")

	args := []string{
		"-F", cfg,
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + muxPath,
		"-o", "ControlPersist=60",
	}
	if opts.Interactive {
		args = append(args, "-t")
	}
	target := opts.Machine + "@orb"
	if opts.User != "" {
		target = opts.User + "@" + target
	}
	args = append(args, target)

	// Build remote command.
	var remote string
	if len(opts.Argv) == 0 {
		// Interactive login shell — let ssh handle it (no command).
		return args, nil
	}
	for _, e := range opts.Env {
		remote += "export " + shellQuote(e) + "; "
	}
	if opts.Workdir != "" {
		remote += "cd " + shellQuote(opts.Workdir) + " && "
	}
	remote += "exec"
	for _, a := range opts.Argv {
		remote += " " + shellQuote(a)
	}
	args = append(args, "--", "bash", "-lc", remote)
	return args, nil
}

// Run invokes ssh against the OrbStack proxy with the given options.
// stdio is inherited; the exit code is propagated by the caller via the
// returned *exec.ExitError.
func Run(opts SSHOptions) error {
	args, err := sshArgs(opts)
	if err != nil {
		return err
	}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Capture runs ssh and captures combined stdout for a non-interactive command.
func Capture(opts SSHOptions) (string, error) {
	opts.Interactive = false
	args, err := sshArgs(opts)
	if err != nil {
		return "", err
	}
	out, err := exec.Command("ssh", args...).Output()
	return string(out), err
}

// shellQuote wraps s for safe inclusion in a single-quoted bash string.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// safe characters
	safe := true
	for _, r := range s {
		switch {
		case 'a' <= r && r <= 'z',
			'A' <= r && r <= 'Z',
			'0' <= r && r <= '9',
			r == '/' || r == '_' || r == '-' || r == '.' || r == ':' || r == '=' || r == ',' || r == '+' || r == '@':
		default:
			safe = false
		}
		if !safe {
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
