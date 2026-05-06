// Package runner dispatches a Provider to a target environment: host,
// shared agent VM, or dedicated per-project VM. Phase 1 only ships
// HostRunner; the remaining modes return errors until later phases.
package runner

import (
	"fmt"

	"github.com/vladzaharia/dotfiles-helpers/agent/args"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
)

// Mode selects which Runner implementation to use.
type Mode int

const (
	ModeHost Mode = iota
	ModeShared
	ModeDedicated
)

// Options carries the agent-helper-side knobs derived from CLI flags
// and config defaults.
type Options struct {
	Mode    Mode
	Local   bool
	Keep    bool
	Rm      bool
	VM      string // override VM name
	NoSync  bool
	Verbose bool

	// LocalModel, when set with Local=true, pins a specific LM Studio model.
	// Empty = auto-pick (Phase 7).
	LocalModel string
	// LocalURL is the LM Studio base URL.
	LocalURL string

	// ExtraPacks are cloud-init packs to install in addition to the
	// agent's required ones (e.g. ["python", "rust"]).
	ExtraPacks []string
}

// Runner executes a Provider with passthrough args under a given strategy.
type Runner interface {
	Run(p provider.Provider, args []string, opts Options) error
}

// PickConfig captures the bits Pick needs that don't fit on Options
// (config-derived data like mount roots).
type PickConfig struct {
	MountRoots []orb.MountSpec
}

// Pick returns the Runner appropriate for the given mode.
func Pick(mode Mode, cfg PickConfig) (Runner, error) {
	switch mode {
	case ModeHost:
		return HostRunner{}, nil
	case ModeShared:
		return SharedVMRunner{MountRoots: cfg.MountRoots}, nil
	case ModeDedicated:
		return DedicatedVMRunner{}, nil
	default:
		return nil, fmt.Errorf("unknown runner mode %d", mode)
	}
}

// ModeFromIsolation maps a parsed --isolated value (or unset) to a Mode,
// using `defaultMode` when unset.
func ModeFromIsolation(iso args.Isolation, defaultMode Mode) Mode {
	switch iso {
	case args.IsolationNone:
		return ModeHost
	case args.IsolationShared:
		return ModeShared
	case args.IsolationFull:
		return ModeDedicated
	default:
		return defaultMode
	}
}
