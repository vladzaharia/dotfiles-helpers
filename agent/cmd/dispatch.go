package cmd

import (
	"fmt"
	"os"

	"github.com/vladzaharia/dotfiles-helpers/agent/args"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/agent/runner"
	"github.com/vladzaharia/dotfiles-helpers/agent/state"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

func defaultModeFromConfig(s string) runner.Mode {
	switch s {
	case "shared":
		return runner.ModeShared
	case "none":
		return runner.ModeHost
	default: // "full" or unknown → safest default
		return runner.ModeDedicated
	}
}

// dispatch is the shared launch path used by every agent subcommand.
// It parses agent-helper flags out of rawArgs, picks a Runner, and
// invokes it. Subcommand-specific tweaks (e.g. local.go forcing Local)
// can mutate the Parsed flags via the optional `mutate` callback.
func dispatch(providerName string, rawArgs []string, mutate func(*args.Parsed)) error {
	p, err := provider.Resolve(providerName)
	if err != nil {
		return err
	}
	parsed, err := args.Parse(rawArgs)
	if err != nil {
		return err
	}
	if mutate != nil {
		mutate(&parsed)
	}
	if err := parsed.Validate(); err != nil {
		return err
	}

	cwdForCfg, _ := os.Getwd()
	cfg := loadEffectiveConfig(cwdForCfg)

	// Apply persisted secrets so EnvForward picks them up like any
	// other host env var. The user's actual shell env always wins.
	if secrets, err := LoadSecrets(); err == nil {
		secrets.ApplyEnv()
	}

	// Auto-extract Claude credentials from macOS Keychain when missing
	// or stale, so VMs can authenticate without a manual `auth claude
	// export-keychain` step. Soft-fail: real auth errors surface inside
	// the agent if this didn't help.
	if providerName == "claude" {
		if err := EnsureClaudeCredentials(); err != nil {
			output.Warn("Claude credential refresh: %v", err)
		}
	}
	defaultMode := defaultModeFromConfig(cfg.Defaults.Isolated)
	// --local always implies host execution (LM Studio is on host loopback).
	if parsed.Local && parsed.Isolated == args.IsolationUnset {
		parsed.Isolated = args.IsolationNone
	}
	mode := runner.ModeFromIsolation(parsed.Isolated, defaultMode)

	// Graceful degradation: if a VM mode is chosen but OrbStack isn't
	// usable, downgrade to host. Explicit user intent is respected:
	// when the user passed --isolated=shared|full, error out loudly so
	// they know to install OrbStack or pick another mode.
	if (mode == runner.ModeShared || mode == runner.ModeDedicated) && !orb.IsUsable() {
		why := orb.UnusableReason()
		if parsed.Isolated == args.IsolationUnset {
			output.Warn("OrbStack not available (%s); running on host", why)
			mode = runner.ModeHost
		} else {
			return fmt.Errorf("--isolated=%s requested but OrbStack is unavailable: %s", parsed.Isolated, why)
		}
	}

	// Self-heal stale session markers before opening a new one.
	state.PruneStale()

	r, err := runner.Pick(mode, runner.PickConfig{
		MountRoots: orb.SharedRoots(cfg.Orb.MountRoots),
	})
	if err != nil {
		return err
	}

	// Track this invocation as a session so concurrent launches can be
	// counted by the prune logic.
	cwd := cwdForCfg
	_ = state.OpenSession(state.Session{
		PID:   os.Getpid(),
		Agent: providerName,
		CWD:   cwd,
	})
	defer state.CloseSession(os.Getpid())

	extraPacks := resolveProjectPacks(cfg.Orb.DefaultPacks, cwd)

	return r.Run(p, parsed.Args, runner.Options{
		Mode:       mode,
		Local:      parsed.Local,
		Keep:       parsed.Keep,
		Rm:         parsed.Rm,
		VM:         parsed.VM,
		NoSync:     parsed.NoSync,
		Verbose:    parsed.Verbose,
		LocalURL:   cfg.Local.URL,
		LocalModel: cfg.Local.DefaultModel,
		ExtraPacks: extraPacks,
	})
}
