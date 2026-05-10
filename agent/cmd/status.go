package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show provider status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	fmt.Println()
	fmt.Println("  " + output.SectionHeader("Providers", 56))
	statuses := []provider.Status{
		provider.DetectClaude(),
		provider.DetectCodex(),
		provider.DetectLMStudio(cfg.Local.URL),
	}
	ollama := provider.DetectOllama()
	if ollama.Installed {
		statuses = append(statuses, ollama)
	}
	provider.PrintStatus(statuses)
	fmt.Println()
	return nil
}

// Config / loadConfig live in config.go.
