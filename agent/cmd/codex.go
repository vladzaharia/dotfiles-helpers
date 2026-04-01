package cmd

import (
	"os"

	"github.com/spf13/cobra"
	iexec "github.com/vladzaharia/dotfiles-helpers/internal/exec"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var codexCmd = &cobra.Command{
	Use:                "codex [flags]",
	Short:              "Launch Codex CLI (OpenAI subscription)",
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if missing := iexec.ValidateDeps("codex"); len(missing) > 0 {
			output.Error("codex CLI not found — install with: brew install codex")
			os.Exit(1)
		}
		output.Info("Launching Codex CLI")
		return iexec.Exec("codex", args)
	},
}
