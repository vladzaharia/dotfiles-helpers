package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/state"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "Manage agent-helper OrbStack machines",
}

var vmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List OrbStack machines (with agent-helper bootstrap state)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !orb.IsInstalled() {
			return fmt.Errorf("orb CLI not found — install OrbStack first")
		}
		machines, err := orb.List()
		if err != nil {
			return err
		}
		if len(machines) == 0 {
			fmt.Println("(no machines)")
			return nil
		}
		fmt.Printf("%-20s %-10s %-10s %-10s %-12s\n", "NAME", "DISTRO", "ARCH", "STATE", "ISOLATED")
		for _, m := range machines {
			iso := "no"
			if m.Config.Isolated {
				iso = "yes"
			}
			fmt.Printf("%-20s %-10s %-10s %-10s %-12s\n",
				m.Name, m.Image.Distro, m.Image.Arch, m.State, iso)
		}
		return nil
	},
}

var vmShellCmd = &cobra.Command{
	Use:   "shell <name>",
	Short: "Open an interactive shell inside an OrbStack machine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return orb.Run(orb.SSHOptions{Machine: args[0], Interactive: true})
	},
}

var vmResetCmd = &cobra.Command{
	Use:   "reset <name>",
	Short: "Delete and recreate an agent VM (forces re-provisioning)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		m, err := orb.Find(name)
		if err != nil {
			return err
		}
		if m == nil {
			return fmt.Errorf("machine %q not found", name)
		}
		output.Warn("Deleting %s …", name)
		return orb.Delete(name)
	},
}

var vmPruneAuto bool

var vmPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete idle agent-helper VMs (no live sessions, idle > shared_idle_days)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		state.PruneStale()
		idleAfter := time.Duration(cfg.Orb.SharedIdleDays) * 24 * time.Hour
		cands, err := state.PruneCandidates(idleAfter)
		if err != nil {
			return err
		}
		if len(cands) == 0 {
			fmt.Println("nothing to prune")
			return nil
		}
		for _, c := range cands {
			if !vmPruneAuto {
				fmt.Printf("would prune %s (idle %s)\n", c.Name, c.IdleFor)
				continue
			}
			output.Info("Pruning %s (idle %s)…", c.Name, c.IdleFor)
			if err := orb.Delete(c.Name); err != nil {
				output.Warn("delete %s: %v", c.Name, err)
				continue
			}
			_ = state.ForgetVM(c.Name)
		}
		return nil
	},
}

func init() {
	vmPruneCmd.Flags().BoolVar(&vmPruneAuto, "auto", false, "actually delete (default: dry run)")
	vmCmd.AddCommand(vmListCmd, vmShellCmd, vmResetCmd, vmPruneCmd)
}
