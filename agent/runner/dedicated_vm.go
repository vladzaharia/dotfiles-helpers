package runner

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/agent/state"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// DedicatedVMRunner runs the provider in a per-project ephemeral VM. The
// CWD is mounted into the VM at /work/<basename>. Default behavior is to
// delete the VM after the session; --keep persists it; --rm forces
// deletion even if a previous --keep created the VM.
type DedicatedVMRunner struct{}

// dedicatedName returns a stable VM name derived from the absolute CWD.
func dedicatedName(cwd string) (string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(abs))
	return "proj-" + hex.EncodeToString(sum[:])[:10], nil
}

func (r DedicatedVMRunner) Run(p provider.Provider, passthrough []string, opts Options) error {
	if !orb.IsUsable() {
		return fmt.Errorf("OrbStack unavailable: %s", orb.UnusableReason())
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getcwd: %w", err)
	}
	mount, vmCwd, err := orb.DedicatedMount(cwd)
	if err != nil {
		return err
	}

	name := opts.VM
	if name == "" {
		if name, err = dedicatedName(cwd); err != nil {
			return err
		}
	}

	machine, err := orb.Find(name)
	if err != nil {
		return err
	}
	plan, err := p.Bootstrap(opts.ExtraPacks)
	if err != nil {
		return err
	}
	created := false
	if machine == nil {
		output.Info("Creating dedicated VM %q for %s (bootstrap %s)…", name, filepath.Base(cwd), plan.Version)
		mounts := []orb.MountSpec{mount}
		mounts = append(mounts, orb.StateMounts(p.StateDirs())...)
		if err := orb.Create(orb.CreateOptions{
			Distro:   "ubuntu",
			Name:     name,
			Isolated: true,
			Mounts:   mounts,
			UserData: plan.Yaml,
		}); err != nil {
			return err
		}
		output.Info("Waiting for cloud-init…")
		if err := orb.WaitCloudInit(name); err != nil {
			return fmt.Errorf("provisioning %s: %w", name, err)
		}
		created = true
	} else {
		if !machine.IsRunning() {
			output.Info("Starting %s…", name)
			if err := orb.Start(name); err != nil {
				return err
			}
		}
		// Auto-reseed on bootstrap drift (only matters when --keep was used previously).
		current, _ := orb.ReadBootstrapVersion(name)
		if current != plan.Version {
			output.Info("Updating VM (bootstrap %s → %s)…", current, plan.Version)
			if err := orb.Reseed(name, plan.Runcmd, plan.Version); err != nil {
				output.Warn("reseed %s: %v (continuing)", name, err)
			}
		}
	}

	// Decide whether to delete on exit.
	// Default: ephemeral — delete unless --keep.
	// --rm always wins (delete).
	shouldDelete := opts.Rm || (created && !opts.Keep) || (!created && opts.Rm)
	if !created && !opts.Keep && !opts.Rm {
		// VM existed from a previous --keep run; honor that and persist.
		shouldDelete = false
	}

	_ = state.TouchVM(name, p.Name(), plan.Version)
	defer state.ReleaseVM(name)
	defer func() {
		if shouldDelete {
			output.Info("Deleting %s…", name)
			if err := orb.Delete(name); err != nil {
				output.Warn("delete %s failed: %v", name, err)
				return
			}
			_ = state.ForgetVM(name)
		}
	}()

	finalArgs := append([]string{p.Binary()}, p.DefaultArgs()...)
	finalArgs = append(finalArgs, passthrough...)

	output.Info("Launching %s in %s (cwd → %s)", p.DisplayName(), name, vmCwd)
	return orb.Run(orb.SSHOptions{
		Machine:     name,
		Interactive: true,
		Workdir:     vmCwd,
		Argv:        finalArgs,
		Env:         collectForwardedEnv(p.EnvForward()),
	})
}
