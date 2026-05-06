package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect or edit the agent-helper TOML config",
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open ~/.config/agent-helper/config.toml in $EDITOR",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.Path("agent-helper")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Materialize a starter config from the in-memory defaults.
			cfg := loadConfig()
			if err := config.Save("agent-helper", &cfg); err != nil {
				return fmt.Errorf("create starter config: %w", err)
			}
			output.Info("Created %s", path)
		}
		editor := os.Getenv("VISUAL")
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			editor = "nano"
		}
		c := exec.Command(editor, path)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the effective config (global + per-project overrides)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		cfg := loadEffectiveConfig(cwd)
		fmt.Printf("# %s\n", config.Path("agent-helper"))
		if path := findProjectConfig(cwd); path != "" {
			fmt.Printf("# overlaid by %s\n", path)
		}
		return toml.NewEncoder(os.Stdout).Encode(cfg)
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path of the global config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(config.Path("agent-helper"))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configEditCmd, configShowCmd, configPathCmd)
}
