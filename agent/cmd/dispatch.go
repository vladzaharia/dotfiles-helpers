package cmd

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/vladzaharia/dotfiles-helpers/agent/args"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/agent/runner"
	"github.com/vladzaharia/dotfiles-helpers/agent/state"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// interactiveStdio reports whether dispatch can prompt the user. Both
// stdin and stderr need to be TTYs so huh forms render correctly.
func interactiveStdio() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

// mergeUnique returns a + (b - a) preserving order, no duplicates.
func mergeUnique(a, b []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

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

	// First-use per-directory: when this project doesn't yet have an
	// .agent-helper.toml AND we're interactive AND the user didn't pass
	// --no-detect, run the project setup wizard so isolation + packs
	// match the project before we provision any VM.
	if !noDetect && interactiveStdio() && findProjectConfig(cwdForCfg) == "" && cwdForCfg != "" {
		shared, _ := orb.Find(providerName)
		iso, packs, saved, err := PromptProjectSetup(cwdForCfg, cfg.Orb.DefaultPacks, shared != nil, cfg.Orb.MountRoots)
		if err != nil {
			output.Warn("project setup: %v", err)
		} else if saved {
			// Re-load effective config so the new file wins.
			cfg = loadEffectiveConfig(cwdForCfg)
		} else {
			// Use detected values for this dispatch only.
			cfg.Defaults.Isolated = iso
			cfg.Orb.DefaultPacks = packs
		}
	} else if !noDetect {
		// Non-interactive or --no-detect=false but project config is
		// known: still augment cfg.Orb.DefaultPacks with project signals
		// so VMs get the right toolchain even without the prompt.
		detectedPacks, _ := orb.DetectProjectPacks(cwdForCfg)
		if len(detectedPacks) > 0 {
			cfg.Orb.DefaultPacks = mergeUnique(cfg.Orb.DefaultPacks, detectedPacks)
		}
	}

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

	// Pack-vs-shared-VM mismatch check. Fires only when this dispatch
	// resolved to ModeShared. We compare requested packs against the
	// shared VM's recorded pack list; if anything is missing, prompt
	// the user (or in non-interactive contexts, fall back to ModeDedicated
	// silently with a warn).
	if mode == runner.ModeShared {
		requested := mergeUnique(cfg.Orb.DefaultPacks, resolveProjectPacks(nil, cwdForCfg))
		missing := MissingPacks(providerName, requested)
		if len(missing) > 0 {
			if !interactiveStdio() {
				output.Warn("shared VM missing packs %v; falling back to dedicated VM for this run", missing)
				mode = runner.ModeDedicated
			} else {
				action, perr := PromptSharedMismatch(providerName, missing, cwdForCfg)
				if perr != nil {
					return perr
				}
				switch action {
				case MismatchAddToShared:
					// Delete the shared VM so the next provision step
					// recreates it with the union of existing + new
					// packs. The runner picks up the merged set via
					// opts.ExtraPacks computed below.
					output.Info("Re-provisioning shared VM with new packs (%v)", missing)
					if err := orb.Delete(providerName); err != nil {
						output.Warn("delete shared VM: %v", err)
					}
					_ = state.ForgetVM(providerName)
				case MismatchUseFull:
					mode = runner.ModeDedicated
				case MismatchSaveFull:
					mode = runner.ModeDedicated
					if cwdForCfg != "" {
						if err := saveProjectConfig(cwdForCfg, "full", cfg.Orb.DefaultPacks); err != nil {
							output.Warn("save .agent-helper.toml: %v", err)
						}
					}
				case MismatchCancel:
					return fmt.Errorf("dispatch cancelled (shared VM missing packs %v)", missing)
				}
			}
		}
	}

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
