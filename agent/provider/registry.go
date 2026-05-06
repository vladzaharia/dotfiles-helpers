package provider

import (
	"fmt"

	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
)

// Provider describes an agent CLI that agent-helper can launch (claude, codex, …).
//
// Bootstrap (cloud-init YAML + version hash) is sourced from the orb
// package via Bootstrap(); each provider only needs to identify its
// runcmd file by name (matching its canonical Name).
type Provider interface {
	Name() string                                    // canonical id, e.g. "claude"
	DisplayName() string                             // human label, e.g. "Claude Code"
	Binary() string                                  // PATH name to exec
	InstallHint() string                             // human install instructions on missing dep
	DefaultArgs() []string                           // baseline args prepended to user args
	StateDirs() []string                             // host paths to live-mount into VMs
	EnvForward() []string                            // env var names to forward into VM SSH session
	SupportsLocal() bool                             // can be combined with --local
	DetectHost() Status                              // host-side detection result
	Bootstrap(extraPacks []string) (orb.BootstrapPlan, error) // cloud-init plan
}

// Resolve returns the Provider implementation for the given canonical name.
func Resolve(name string) (Provider, error) {
	switch name {
	case "claude":
		return ClaudeProvider{}, nil
	case "codex":
		return CodexProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}

// All returns every registered provider, in stable order.
func All() []Provider {
	return []Provider{ClaudeProvider{}, CodexProvider{}}
}
