package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
)

var packsCmd = &cobra.Command{
	Use:   "packs",
	Short: "List or inspect cloud-init packs available in the VM bootstrap",
}

var packsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Group packs by category",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := orb.AvailablePacks()
		if err != nil {
			return err
		}
		byCat := map[string][]*orb.Pack{}
		for _, p := range all {
			byCat[string(p.Category)] = append(byCat[string(p.Category)], p)
		}
		cats := make([]string, 0, len(byCat))
		for c := range byCat {
			cats = append(cats, c)
		}
		sort.Strings(cats)
		for _, c := range cats {
			fmt.Printf("\n  %s\n", c)
			fmt.Println("  ────────")
			ps := byCat[c]
			sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
			for _, p := range ps {
				deps := ""
				if len(p.Deps) > 0 {
					deps = fmt.Sprintf(" (deps: %v)", p.Deps)
				}
				fmt.Printf("  %-14s %s%s\n", p.Name, p.Description, deps)
			}
		}
		fmt.Println()
		return nil
	},
}

var packsRenderCmd = &cobra.Command{
	Use:   "render <agent> [extra-packs...]",
	Short: "Print the cloud-init YAML that would be used (for debugging)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := args[0]
		plan, err := orb.RenderCloudInit(agent, args[1:])
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "# bootstrap_version=%s\n", plan.Version)
		_, _ = os.Stdout.Write(plan.Yaml)
		return nil
	},
}

func init() {
	packsCmd.AddCommand(packsListCmd, packsRenderCmd)
}
