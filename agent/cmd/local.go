package cmd

import (
	"github.com/spf13/cobra"
	ahargs "github.com/vladzaharia/dotfiles-helpers/agent/args"
)

var localCmd = &cobra.Command{
	Use:                "local [flags]",
	Short:              "Launch Claude Code with local models (LM Studio)",
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Sugar for `agent-helper claude --local`.
		return dispatch("claude", args, func(p *ahargs.Parsed) {
			p.Local = true
			// Local always runs on the host — LM Studio binds to host loopback.
			p.Isolated = ahargs.IsolationNone
		})
	},
}
