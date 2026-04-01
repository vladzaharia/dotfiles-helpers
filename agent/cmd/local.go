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
			output.Error("codex CLI not found — install with: brew install codex")
			os.Exit(1)
		}

		if cfg.Local.Provider == "lmstudio" {
			os.Setenv("OPENAI_BASE_URL", cfg.Local.URL+"/v1")
			os.Setenv("OPENAI_API_KEY", "lmstudio")
		}

		execArgs := []string{"--oss"}
		if cfg.Local.DefaultModel != "" {
			execArgs = append(execArgs, "--model", cfg.Local.DefaultModel)
		}
		execArgs = append(execArgs, args...)

		output.Info("Launching Codex (local) via %s at %s", cfg.Local.Provider, cfg.Local.URL)
		return iexec.Exec("codex", execArgs)
	},
}
