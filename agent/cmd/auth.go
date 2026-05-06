package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// `agent-helper auth` groups credential-management helpers for the
// providers' headless / VM auth.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage agent credentials for VM / headless use",
}

var authClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Inspect or export Claude Code credentials for VM use",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		fmt.Println("  Claude Code credentials")
		fmt.Println("  ───────────────────────")

		if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" {
			fmt.Println(output.StatusOK("CLAUDE_CODE_OAUTH_TOKEN", "set in environment — will be forwarded to VM"))
		} else {
			fmt.Println(output.StatusFail("CLAUDE_CODE_OAUTH_TOKEN", "not set (preferred path for VM auth)"))
		}

		credPath := credentialsPath()
		if st, err := os.Stat(credPath); err == nil && !st.IsDir() {
			fmt.Println(output.StatusOK("~/.claude/.credentials.json", fmt.Sprintf("present (%d bytes) — mounted into VM", st.Size())))
		} else {
			fmt.Println(output.StatusFail("~/.claude/.credentials.json", "absent (file path will be mounted as empty if missing)"))
		}

		if runtime.GOOS == "darwin" {
			if hasKeychainEntry() {
				fmt.Println(output.StatusOK("macOS Keychain", "Claude Code-credentials present"))
			} else {
				fmt.Println(output.StatusFail("macOS Keychain", "no Claude Code-credentials entry"))
			}
		}

		fmt.Println()
		fmt.Println("  To set up VM auth, pick one (in order of preference):")
		fmt.Println()
		fmt.Println("  (a) Long-lived OAuth token (recommended):")
		fmt.Println("        claude setup-token")
		fmt.Println("        # then add to shell rc:")
		fmt.Println("        export CLAUDE_CODE_OAUTH_TOKEN=<token>")
		fmt.Println()
		fmt.Println("  (b) Sync from macOS Keychain to credentials file (one-shot):")
		fmt.Println("        agent-helper auth claude export-keychain")
		fmt.Println()
		return nil
	},
}

var authClaudeExportKeychainCmd = &cobra.Command{
	Use:   "export-keychain",
	Short: "Copy Claude credentials from macOS Keychain to ~/.claude/.credentials.json",
	Long: `On macOS, Claude Code stores OAuth credentials in Keychain. Linux
(and headless VM sessions) instead read ~/.claude/.credentials.json. This
command runs ` + "`security find-generic-password ...`" + ` to extract the JSON blob
and writes it to the file (mode 0600), where the live mount will share
it with any agent-helper-managed VM.

Caveats:
  - The token typically lasts ~6 hours; re-run after refresh.
  - Mac-side ` + "`claude`" + ` may DELETE this file on next launch (known bug
    anthropics/claude-code#10039). The long-lived token path is safer.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("export-keychain is macOS-only")
		}
		json, err := exec.Command("security", "find-generic-password",
			"-a", os.Getenv("USER"),
			"-s", "Claude Code-credentials",
			"-w").Output()
		if err != nil {
			return fmt.Errorf("Keychain entry not found — log in with `claude /login` first: %w", err)
		}
		path := credentialsPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, json, 0o600); err != nil {
			return err
		}
		output.Success("Wrote %s (%d bytes)", path, len(json))
		fmt.Println("  The file is mounted into agent-helper VMs at the same path.")
		fmt.Println("  Re-run if the token expires or mac claude deletes the file.")
		return nil
	},
}

func credentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", ".credentials.json")
}

func hasKeychainEntry() bool {
	err := exec.Command("security", "find-generic-password",
		"-a", os.Getenv("USER"),
		"-s", "Claude Code-credentials").Run()
	return err == nil
}

func init() {
	authCmd.AddCommand(authClaudeCmd)
	authClaudeCmd.AddCommand(authClaudeExportKeychainCmd)
}
