package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
	iexec "github.com/vladzaharia/dotfiles-helpers/internal/exec"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:          "agent-helper",
	Short:        "Unified AI coding agent dispatcher",
	Long:         "Dispatch between Claude Code, Codex CLI, and local models (LM Studio/Ollama).",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists("agent-helper") {
			fmt.Println("No configuration found. Running setup...")
			fmt.Println()
			return runSetup(cmd, args)
		}

		// Smart default: claude if online, local if offline
		if hasInternet() {
			if _, err := iexec.FindBinary("claude"); err == nil {
				output.Info("Launching Claude Code (online)")
				return iexec.Exec("claude", args)
			}
		}

		// Offline or no claude — try local
		cfg := loadConfig()
		if _, err := iexec.FindBinary("codex"); err == nil {
			os.Setenv("OPENAI_BASE_URL", cfg.Local.URL+"/v1")
			os.Setenv("OPENAI_API_KEY", "lmstudio")
			localArgs := []string{"--oss"}
			if cfg.Local.DefaultModel != "" {
				localArgs = append(localArgs, "--model", cfg.Local.DefaultModel)
			}
			localArgs = append(localArgs, args...)
			output.Info("Launching Codex (local) via %s", cfg.Local.Provider)
			return iexec.Exec("codex", localArgs)
		}

		// Nothing available — show status
		output.Warn("No agent available to launch. Install claude or codex.")
		return runStatus(cmd, args)
	},
}

func hasInternet() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Head("https://api.anthropic.com")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(claudeCmd)
	rootCmd.AddCommand(codexCmd)
	rootCmd.AddCommand(localCmd)
	rootCmd.Version = version
}
