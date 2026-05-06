package main

import (
	"github.com/vladzaharia/dotfiles-helpers/agent/cmd"
	"github.com/vladzaharia/dotfiles-helpers/internal/alias"
)

func main() {
	// Shadow alias dispatch: when invoked via a busybox-style symlink whose
	// basename matches a provider name, route to that subcommand.
	alias.RewriteArgs("agent-helper", map[string]string{
		"ag":     "",
		"claude": "claude",
		"codex":  "codex",
		"gemini": "gemini", // future provider
	})
	cmd.Execute()
}
