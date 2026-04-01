package cmd

import (
	"os"

	"github.com/spf13/cobra"
	iexec "github.com/vladzaharia/dotfiles-helpers/internal/exec"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var localCmd = &cobra.Command{
	Use:                "local [flags]",
	Short:              "Launch Claude Code with local models (LM Studio)",
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()

		if missing := iexec.ValidateDeps("claude"); len(missing) > 0 {
			output.Error("Claude Code not found — install with: brew install --cask claude-code")
			os.Exit(1)
		}

		// Point Claude Code at local LM Studio
		os.Setenv("ANTHROPIC_BASE_URL", cfg.Local.URL)
		os.Setenv("ANTHROPIC_API_KEY", "lmstudio")

		execArgs := args
		if cfg.Local.DefaultModel != "" {
			execArgs = append([]string{"--model", cfg.Local.DefaultModel}, execArgs...)
		}

		output.Info("Launching Claude Code (local) via %s at %s", cfg.Local.Provider, cfg.Local.URL)
		return iexec.Exec("claude", execArgs)
	},
}
