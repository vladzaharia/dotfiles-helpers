package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/agent/state"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

var doctorAuto bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose agent-helper environment and prune idle VMs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()

		fmt.Println()
		fmt.Println("  " + output.SectionHeader("OrbStack", 56))
		if !orb.IsInstalled() {
			fmt.Println(output.StatusFail("orb CLI", "not on PATH — install OrbStack"))
		} else if !orb.IsRunning() {
			fmt.Println(output.StatusFail("daemon", "not running — `orb start` to fix"))
		} else {
			fmt.Println(output.StatusOK("daemon", "running"))
		}

		fmt.Println()
		fmt.Println("  " + output.SectionHeader("Providers (host)", 56))
		for _, p := range provider.All() {
			s := p.DetectHost()
			detail := s.Detail
			if !s.Installed {
				detail = p.InstallHint()
			}
			if s.Installed {
				fmt.Println(output.StatusOK(p.DisplayName(), detail))
			} else {
				fmt.Println(output.StatusFail(p.DisplayName(), detail))
			}
		}

		fmt.Println()
		fmt.Println("  " + output.SectionHeader("Auth (VM-ready)", 56))
		printClaudeAuthSummary()

		fmt.Println()
		fmt.Println("  " + output.SectionHeader("Sessions (live)", 56))
		state.PruneStale()
		ses, _ := state.Sessions()
		if len(ses) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, s := range ses {
				fmt.Printf("  pid=%-6d agent=%-7s cwd=%s\n", s.PID, s.Agent, s.CWD)
			}
		}

		fmt.Println()
		fmt.Println("  " + output.SectionHeader("VMs (tracked)", 56))
		vms, _ := state.VMs()
		if len(vms) == 0 {
			fmt.Println("  (none)")
		}
		for _, v := range vms {
			fmt.Printf("  %s  agent=%s  bootstrap=%s  pids=%v  last_active=%s\n",
				v.Name, v.Agent, v.BootstrapVersion, v.OwnerPIDs,
				humanAgo(time.Since(v.LastActive)))
		}

		idleAfter := time.Duration(cfg.Orb.SharedIdleDays) * 24 * time.Hour
		cands, _ := state.PruneCandidates(idleAfter)
		if len(cands) > 0 {
			fmt.Println()
			fmt.Println("  " + output.SectionHeader(fmt.Sprintf("Idle (>%s)", idleAfter), 56))
			for _, c := range cands {
				marker := "(would prune)"
				if doctorAuto {
					marker = "(pruning…)"
				}
				fmt.Printf("  %s  idle=%s  %s\n", c.Name, humanAgo(c.IdleFor), marker)
				if doctorAuto {
					if err := orb.Delete(c.Name); err != nil {
						output.Warn("delete %s: %v", c.Name, err)
						continue
					}
					_ = state.ForgetVM(c.Name)
				}
			}
		}
		fmt.Println()
		return nil
	},
}

func humanAgo(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours())/24)
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorAuto, "auto", false, "actually prune idle VMs (default: dry run)")
}
