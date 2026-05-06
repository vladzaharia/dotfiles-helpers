package cmd

import (
	"github.com/spf13/cobra"
)

var claudeCmd = &cobra.Command{
	Use:                "claude [flags]",
	Short:              "Launch Claude Code (subscription)",
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return dispatch("claude", args, nil)
	},
}
