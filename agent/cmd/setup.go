package cmd

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
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

	// ── Detect host providers ─────────────────────────────────────
	fmt.Println("  Detecting providers...")
	statuses := []provider.Status{
		provider.DetectClaude(),
		provider.DetectCodex(),
		provider.DetectLMStudio(cfg.Local.URL),
	}
	if ollama := provider.DetectOllama(); ollama.Installed {
		statuses = append(statuses, ollama)
	}
	fmt.Println()
	provider.PrintStatus(statuses)
	fmt.Println()

	// ── Defaults ──────────────────────────────────────────────────
	fmt.Println("  Defaults")
	fmt.Println("  ────────")
	cfg.Defaults.Agent = promptString(reader,
		"  Default agent (claude/codex)", cfg.Defaults.Agent)
	cfg.Defaults.Isolated = promptString(reader,
		"  Default isolation (none/shared/full)", cfg.Defaults.Isolated)

	// ── OrbStack ──────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  OrbStack VM defaults")
	fmt.Println("  ────────────────────")
	if orb.IsInstalled() {
		fmt.Println(output.StatusOK("orb CLI", "found"))
	} else {
		fmt.Println(output.StatusFail("orb CLI", "not installed (VM modes will fall back to host)"))
	}
	cfg.Orb.MountRoots = promptCSV(reader,
		"  Mount roots for shared VMs (host paths)", cfg.Orb.MountRoots)
	cfg.Orb.DefaultPacks = promptCSV(reader,
		"  Default packs (e.g. nodejs,python,rust)", cfg.Orb.DefaultPacks)
	cfg.Orb.SharedIdleDays = promptInt(reader,
		"  Idle days before pruning shared VMs", cfg.Orb.SharedIdleDays)

	// ── LM Studio (--local) ───────────────────────────────────────
	fmt.Println()
	fmt.Println("  LM Studio (host-only --local mode)")
	fmt.Println("  ──────────────────────────────────")
	cfg.Local.URL = promptString(reader, "  LM Studio URL", cfg.Local.URL)
	cfg.Local.DefaultModel = promptString(reader,
		"  Default model (empty = auto-pick largest loaded)", cfg.Local.DefaultModel)

	// ── Save before token flow so we don't lose work ──────────────
	if err := config.Save("agent-helper", &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	output.Success("Config saved to %s", config.Path("agent-helper"))

	// ── Claude credentials for VM use ─────────────────────────────
	fmt.Println()
	fmt.Println("  Claude credentials (for VM auth)")
	fmt.Println("  ────────────────────────────────")
	printClaudeAuthSummary()

	if needsClaudeAuthSetup() {
		fmt.Println()
		fmt.Println("  Claude isn't fully set up for VM use yet. Pick one:")
		fmt.Println("    1. Generate a long-lived OAuth token (recommended)")
		if runtime.GOOS == "darwin" && hasKeychainEntry() {
			fmt.Println("    2. Sync from macOS Keychain to a credentials file")
		}
		fmt.Println("    s. Skip")
		choice := promptString(reader, "  Choose", "1")
		switch choice {
		case "1":
			if err := runClaudeSetupToken(reader); err != nil {
				output.Warn("setup-token: %v", err)
			}
		case "2":
			if err := runKeychainExport(); err != nil {
				output.Warn("export-keychain: %v", err)
			}
		}
	}

	fmt.Println()
	output.Success("Setup complete. Try `agent-helper doctor` to verify.")
	fmt.Println()
	return nil
}

func promptString(r *bufio.Reader, label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	in, _ := r.ReadString('\n')
	in = strings.TrimSpace(in)
	if in == "" {
		return def
	}
	return in
}

func promptCSV(r *bufio.Reader, label string, def []string) []string {
	cur := strings.Join(def, ",")
	in := promptString(r, label, cur)
	if in == cur {
		return def
	}
	out := make([]string, 0)
	for _, s := range strings.Split(in, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func promptInt(r *bufio.Reader, label string, def int) int {
	in := promptString(r, label, fmt.Sprintf("%d", def))
	var v int
	if _, err := fmt.Sscanf(in, "%d", &v); err != nil {
		return def
	}
	return v
}

func needsClaudeAuthSetup() bool {
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" {
		return false
	}
	if st, err := os.Stat(credentialsPath()); err == nil && !st.IsDir() {
		return false
	}
	return true
}
