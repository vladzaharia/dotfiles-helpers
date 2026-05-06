package provider

import "github.com/vladzaharia/dotfiles-helpers/agent/orb"

// ClaudeProvider wraps Anthropic's Claude Code CLI.
type ClaudeProvider struct{}

func (ClaudeProvider) Name() string        { return "claude" }
func (ClaudeProvider) DisplayName() string { return "Claude Code" }
func (ClaudeProvider) Binary() string      { return "claude" }
func (ClaudeProvider) InstallHint() string {
	return "install with: brew install --cask claude-code"
}
func (ClaudeProvider) DefaultArgs() []string { return nil }
func (ClaudeProvider) StateDirs() []string {
	return []string{"~/.claude", "~/.claude.json"}
}
func (ClaudeProvider) SupportsLocal() bool { return true }
func (ClaudeProvider) DetectHost() Status  { return DetectClaude() }

func (ClaudeProvider) Bootstrap(extraPacks []string) (orb.BootstrapPlan, error) {
	return orb.RenderCloudInit("claude", extraPacks)
}
