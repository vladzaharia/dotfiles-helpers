package provider

import "github.com/vladzaharia/dotfiles-helpers/agent/orb"

// CodexProvider wraps OpenAI's Codex CLI.
type CodexProvider struct{}

func (CodexProvider) Name() string        { return "codex" }
func (CodexProvider) DisplayName() string { return "Codex CLI" }
func (CodexProvider) Binary() string      { return "codex" }
func (CodexProvider) InstallHint() string {
	return "install with: brew install codex"
}
func (CodexProvider) DefaultArgs() []string { return nil }
func (CodexProvider) StateDirs() []string {
	return []string{"~/.codex"}
}
func (CodexProvider) SupportsLocal() bool { return false }
func (CodexProvider) DetectHost() Status  { return DetectCodex() }

func (CodexProvider) Bootstrap(extraPacks []string) (orb.BootstrapPlan, error) {
	return orb.RenderCloudInit("codex", extraPacks)
}
