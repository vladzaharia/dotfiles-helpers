package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show provider status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	fmt.Println()
	fmt.Println("  Providers")
	fmt.Println("  ─────────")
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

type Config struct {
	Local LocalConfig `toml:"local"`
}

type LocalConfig struct {
	Provider     string `toml:"provider"`
	URL          string `toml:"url"`
	DefaultModel string `toml:"default_model"`
}

func loadConfig() Config {
	cfg := Config{
		Local: LocalConfig{
			Provider: "lmstudio",
			URL:      "http://127.0.0.1:1234",
		},
	}
	_ = config.Load("agent-helper", &cfg)
	return cfg
}
