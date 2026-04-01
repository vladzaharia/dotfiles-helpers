package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "agent-helper",
	Short: "Unified AI coding agent dispatcher",
	Long:  "Dispatch between Claude Code, Codex CLI, and local models (LM Studio/Ollama).",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists("agent-helper") {
			fmt.Println("No configuration found. Running setup...")
			fmt.Println()
			return runSetup(cmd, args)
		}
		return runStatus(cmd, args)
	},
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
