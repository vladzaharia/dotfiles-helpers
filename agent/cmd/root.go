package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:                "agent-helper",
	Short:              "Unified AI coding agent dispatcher",
	Long:               "Dispatch between Claude Code, Codex CLI, and local models (LM Studio/Ollama). Run inside isolated OrbStack VMs by default.",
	SilenceUsage:       true,
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists("agent-helper") {
			fmt.Println("No configuration found. Running setup...")
			fmt.Println()
			return runSetup(cmd, args)
		}
		// Use os.Args[1:] because cobra's `args` drops unknown flags even
		// with FParseErrWhitelist.UnknownFlags=true.
		cwd, _ := os.Getwd()
		cfg := loadEffectiveConfig(cwd)
		return dispatch(cfg.Defaults.Agent, os.Args[1:], nil)
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
	rootCmd.AddCommand(vmCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(packsCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.PersistentFlags().BoolVar(&refreshClaudeCreds, "refresh-creds", false,
		"Re-extract Claude credentials from macOS Keychain (ignore cached ~/.claude/.credentials.json)")
	rootCmd.Version = version
}
