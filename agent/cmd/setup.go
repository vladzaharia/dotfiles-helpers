package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard",
	RunE:  runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	output.Info("Agent Helper Setup")
	fmt.Println()

	// Detect providers
	fmt.Println("  Detecting providers...")
	statuses := []provider.Status{
		provider.DetectClaude(),
		provider.DetectCodex(),
		provider.DetectLMStudio(cfg.Local.URL),
	}
	ollama := provider.DetectOllama()
	if ollama.Installed {
		statuses = append(statuses, ollama)
	}
	fmt.Println()
	provider.PrintStatus(statuses)
	fmt.Println()

	// Configure local provider
	fmt.Printf("  LM Studio URL [%s]: ", cfg.Local.URL)
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		cfg.Local.URL = strings.TrimSpace(input)
	}

	fmt.Printf("  Default local model [%s]: ", cfg.Local.DefaultModel)
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		cfg.Local.DefaultModel = strings.TrimSpace(input)
	}

	// Choose default provider
	if ollama.Installed {
		fmt.Printf("  Local provider (lmstudio/ollama) [%s]: ", cfg.Local.Provider)
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			cfg.Local.Provider = strings.TrimSpace(input)
		}
	}

	// Save config
	if err := config.Save("agent-helper", &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	output.Success("Config saved to %s", config.Path("agent-helper"))
	fmt.Println()

	return nil
}
