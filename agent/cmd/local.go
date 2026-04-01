package cmd

import (
	"os"

	"github.com/spf13/cobra"
	iexec "github.com/vladzaharia/dotfiles-helpers/internal/exec"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var localCmd = &cobra.Command{
	Use:                "local [flags]",
	Short:              "Launch Codex CLI with local models (LM Studio/Ollama)",
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()

		if missing := iexec.ValidateDeps("codex"); len(missing) > 0 {
			output.Error("codex CLI not found — install with: brew install --cask codex")
			os.Exit(1)
		}

		// Pass LM Studio config via codex CLI flags (not deprecated env vars)
		execArgs := []string{"--oss"}
		if cfg.Local.Provider == "lmstudio" {
			execArgs = append(execArgs, "--local-provider", "lmstudio")
			execArgs = append(execArgs, "-c", "openai_base_url="+cfg.Local.URL+"/v1")
		}
		if cfg.Local.DefaultModel != "" {
			execArgs = append(execArgs, "--model", cfg.Local.DefaultModel)
		}
		execArgs = append(execArgs, args...)

		output.Info("Launching Codex (local) via %s at %s", cfg.Local.Provider, cfg.Local.URL)
		return iexec.Exec("codex", execArgs)
	},
}
