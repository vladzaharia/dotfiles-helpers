package cmd

import (
	"os"

	"github.com/spf13/cobra"
	iexec "github.com/vladzaharia/dotfiles-helpers/internal/exec"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var claudeCmd = &cobra.Command{
	Use:                "claude [flags]",
	Short:              "Launch Claude Code (subscription)",
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if missing := iexec.ValidateDeps("claude"); len(missing) > 0 {
			output.Error("Claude Code not found — install with: brew install --cask claude-code")
			os.Exit(1)
		}
		output.Info("Launching Claude Code")
		return iexec.Exec("claude", args)
	},
}
