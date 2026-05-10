package cmd

import (
	"os"

	"github.com/spf13/cobra"
	ahargs "github.com/vladzaharia/dotfiles-helpers/agent/args"
)

var localCmd = &cobra.Command{
	Use:                "local [flags]",
	Short:              "Launch Claude Code with local models (LM Studio)",
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Pre-flight: every --local startup checks LM Studio
		// reachability and walks the user through fixes if it's not
		// available. First-time invocations also see a one-shot
		// explainer about isolation behavior.
		cwd, _ := os.Getwd()
		cfg := loadEffectiveConfig(cwd)
		if interactiveStdio() {
			if err := RunLMStudioPreflight(cmd.Context(), cfg.Local.URL); err != nil {
				return err
			}
		}
		// Sugar for `agent-helper claude --local`.
		return dispatch("claude", args, func(p *ahargs.Parsed) {
			p.Local = true
			// Local always runs on the host — LM Studio binds to host loopback.
			p.Isolated = ahargs.IsolationNone
		})
	},
}
