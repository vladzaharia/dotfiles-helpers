package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// credentialsCacheTTL is how old ~/.claude/.credentials.json can be
// before EnsureClaudeCredentials re-extracts from macOS Keychain. Claude
// OAuth tokens issued via setup-token last ~6h, but Keychain-stored
// credentials sometimes refresh in place — re-pulling once a day keeps
// us close enough to fresh without paying the security(1) cost on every
// dispatch.
const credentialsCacheTTL = 24 * time.Hour

// refreshClaudeCreds is set by the global --refresh-creds flag (registered
// in root.go) and read by EnsureClaudeCredentials.
var refreshClaudeCreds bool

// EnsureClaudeCredentials makes sure Claude can authenticate inside a
// VM. The VM live-mounts ~/.claude/.credentials.json, so we just need
// to populate that file from whatever credential source is available on
// the host:
//
//  1. CLAUDE_CODE_OAUTH_TOKEN env var → already forwarded by EnvForward;
//     no file needed. We early-return.
//  2. ~/.claude/.credentials.json present and recent → trusted as-is.
//  3. macOS Keychain entry present → extract via `security
//     find-generic-password -w` and write the file (mode 0600).
//
// Linux and macOS-without-Keychain-entry paths are no-ops; callers
// should not block dispatch on this — agent-level auth errors will
// surface at runtime if the VM session lacks credentials.
func EnsureClaudeCredentials() error {
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" {
		return nil
	}
	path := credentialsPath()
	if refreshClaudeCreds {
		_ = os.Remove(path)
	} else if st, err := os.Stat(path); err == nil && !st.IsDir() {
		if time.Since(st.ModTime()) < credentialsCacheTTL {
			return nil
		}
	}
	if runtime.GOOS != "darwin" {
		return nil
	}
	if !hasKeychainEntry() {
		return nil
	}
	return extractKeychainCreds(path)
}

// extractKeychainCreds runs `security find-generic-password ... -w` and
// writes the result to dst with mode 0600.
func extractKeychainCreds(dst string) error {
	out, err := exec.Command("security", "find-generic-password",
		"-a", os.Getenv("USER"),
		"-s", "Claude Code-credentials",
		"-w").Output()
	if err != nil {
		return fmt.Errorf("keychain extraction (security find-generic-password): %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, out, 0o600)
}
