package cmd

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// Re-run subcommands let the user tweak one section without walking
// the whole wizard. Each command loads config, runs a focused subset
// of the same prompts, and saves.

var setupDefaultsCmd = &cobra.Command{
	Use:   "defaults",
	Short: "Re-run just the default agent + isolation prompts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		claude := provider.DetectClaude()
		codex := provider.DetectCodex()

		agentOpts := []huh.Option[string]{}
		if claude.Installed {
			agentOpts = append(agentOpts, huh.NewOption("Claude Code", "claude"))
		}
		if codex.Installed {
			agentOpts = append(agentOpts, huh.NewOption("Codex CLI", "codex"))
		}
		if len(agentOpts) == 0 {
			agentOpts = []huh.Option[string]{
				huh.NewOption("Claude Code (install pending)", "claude"),
				huh.NewOption("Codex CLI (install pending)", "codex"),
			}
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Default agent").
					Options(agentOpts...).
					Value(&cfg.Defaults.Agent),
				huh.NewSelect[string]().
					Title("Default isolation").
					DescriptionFunc(func() string {
						return isolationDescription(cfg.Defaults.Isolated)
					}, &cfg.Defaults.Isolated).
					Options(
						huh.NewOption("none — run on host", "none"),
						huh.NewOption("shared — long-lived per-provider VM", "shared"),
						huh.NewOption("full — dedicated ephemeral VM", "full"),
					).
					Value(&cfg.Defaults.Isolated),
			),
		)
		if err := runForm(form); err != nil {
			return err
		}
		return saveAndReport(&cfg)
	},
}

var setupProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Re-detect Claude / Codex and update binary path overrides",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		// Show fresh detection.
		statuses := []provider.Status{
			provider.DetectClaude(),
			provider.DetectCodex(),
		}
		fmt.Println()
		provider.PrintStatus(statuses)
		fmt.Println()
		if err := promptProviderOverrides(&cfg); err != nil {
			return err
		}
		return saveAndReport(&cfg)
	},
}

var setupOrbCmd = &cobra.Command{
	Use:   "orb",
	Short: "Re-run OrbStack install/check + pack & mount config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		if err := promptOrbStackInstall(ctx); err != nil {
			output.Warn("OrbStack: %v", err)
			return saveAndReport(&cfg)
		}
		if !orb.IsInstalled() {
			output.Warn("OrbStack still not installed. Re-run `agent-helper setup orb` after manual install.")
			return saveAndReport(&cfg)
		}
		if err := promptPacksAndMounts(&cfg); err != nil {
			return err
		}
		return saveAndReport(&cfg)
	},
}

var setupLMStudioCmd = &cobra.Command{
	Use:   "lmstudio",
	Short: "Re-run the LM Studio URL + per-tier model picker",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		if err := promptLMStudio(ctx, &cfg); err != nil {
			output.Warn("LM Studio: %v", err)
		}
		return saveAndReport(&cfg)
	},
}

var setupClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Re-run Claude credential setup (browser OAuth or Keychain export)",
	RunE: func(cmd *cobra.Command, args []string) error {
		printClaudeAuthSummary()
		if err := promptClaudeAuth(); err != nil {
			output.Warn("Claude auth: %v", err)
		}
		return nil
	},
}

func init() {
	setupCmd.AddCommand(setupDefaultsCmd)
	setupCmd.AddCommand(setupProvidersCmd)
	setupCmd.AddCommand(setupOrbCmd)
	setupCmd.AddCommand(setupLMStudioCmd)
	setupCmd.AddCommand(setupClaudeCmd)
}
