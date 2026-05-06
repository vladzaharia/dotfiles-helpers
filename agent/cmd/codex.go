package cmd

import (
	"github.com/spf13/cobra"
)

var codexCmd = &cobra.Command{
	Use:                "codex [flags]",
	Short:              "Launch Codex CLI (OpenAI subscription)",
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return dispatch("codex", args, nil)
	},
}
