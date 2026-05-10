package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/agent/state"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// SharedVMRunner runs the provider in a long-lived OrbStack machine
// shared by every project that uses this agent. The machine has a
// fixed name (== provider.Name()) and is created with the configured
// mount roots so that CWDs underneath them are live-shared with the host.
type SharedVMRunner struct {
	MountRoots []orb.MountSpec
}

func (r SharedVMRunner) Run(p provider.Provider, passthrough []string, opts Options) error {
	if !orb.IsUsable() {
		return fmt.Errorf("OrbStack unavailable: %s", orb.UnusableReason())
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getcwd: %w", err)
	}
	vmCwd, ok := orb.ResolveCWD(cwd, r.MountRoots)
	if !ok {
		return fmt.Errorf(
			"cwd %s is not under any configured mount_root — add it to [orb] mount_roots, "+
				"or use --isolated=full / --isolated=none", cwd)
	}

	name := opts.VM
	if name == "" {
		name = p.Name()
	}

	plan, err := p.Bootstrap(opts.ExtraPacks)
	if err != nil {
		return err
	}
	mounts := append([]orb.MountSpec{}, r.MountRoots...)
	mounts = append(mounts, orb.StateMounts(p.StateDirs())...)
	expectedSig := orb.MountSignature(mounts)

	machine, err := orb.Find(name)
	if err != nil {
		return err
	}

	// Mount drift: orb create --mount is create-time only, so a
	// mount-config change requires deleting and recreating the VM.
	if machine != nil {
		currentSig, _ := orb.ReadMountSignature(name)
		if currentSig != "" && currentSig != expectedSig {
			output.Info("Mount config changed (%s → %s); recreating %s…", currentSig, expectedSig, name)
			if err := orb.Delete(name); err != nil {
				return fmt.Errorf("delete %s: %w", name, err)
			}
			_ = state.ForgetVM(name)
			machine = nil
		}
	}

	if machine == nil {
		output.Info("Creating shared VM %q (bootstrap %s)…", name, plan.Version)
		err := provisionGuard(name,
			func(ctx context.Context) error {
				return orb.CreateContext(ctx, orb.CreateOptions{
					Distro:   "ubuntu",
					Name:     name,
					Isolated: true,
					Mounts:   mounts,
					UserData: plan.Yaml,
				})
			},
			func(ctx context.Context) error {
				output.Info("Waiting for cloud-init…")
				return orb.WaitCloudInitContext(ctx, name)
			},
		)
		if err != nil {
			return err // provisionGuard already cleaned up the partial VM
		}
		_ = orb.WriteMountSignature(name, expectedSig)
		// Track which packs were provisioned so future dispatches can
		// detect mismatch when a new project requests a pack the VM
		// doesn't have.
		_ = state.RecordVMPacks(name, opts.ExtraPacks)
	} else {
		if !machine.IsRunning() {
			output.Info("Starting %s…", name)
			if err := orb.Start(name); err != nil {
				return err
			}
		}
		// Auto-reseed on bootstrap drift (runcmd-only changes).
		current, _ := orb.ReadBootstrapVersion(name)
		if current != plan.Version {
			output.Info("Updating VM (bootstrap %s → %s)…", current, plan.Version)
			if err := orb.Reseed(name, plan.Runcmd, plan.Version); err != nil {
				output.Warn("reseed %s: %v (continuing)", name, err)
			}
		}
	}

	_ = state.TouchVM(name, p.Name(), plan.Version)
	defer state.ReleaseVM(name)

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
