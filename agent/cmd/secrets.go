package cmd

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
)

// Secrets holds tokens that we want to forward into VM SSH sessions
// without persisting them in the user's shell rc. Stored at
// ~/.config/agent-helper/secrets.toml with mode 0600.
//
// On every agent-helper invocation we apply the file into os.Environ
// so EnvForward picks it up like any other host env var. If the user
// already has the corresponding env var set in their shell, we don't
// override it.
type Secrets struct {
	ClaudeOAuthToken string `toml:"claude_oauth_token"`
	OpenAIAPIKey     string `toml:"openai_api_key"`
}

func secretsPath() string {
	return filepath.Join(config.Dir("agent-helper"), "secrets.toml")
}

// LoadSecrets reads the secrets file. Missing file returns an empty
// Secrets and no error.
func LoadSecrets() (Secrets, error) {
	var s Secrets
	path := secretsPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return s, nil
	}
	_, err := toml.DecodeFile(path, &s)
	return s, err
}

// ensureSecretsWritable creates the config directory if missing and
// confirms a 0600 file can be written there. Used as a pre-flight before
// long interactive credential flows so we fail fast on permission/disk
// issues rather than after the user has authorized in a browser.
func ensureSecretsWritable() error {
	dir := config.Dir("agent-helper")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	probe := filepath.Join(dir, ".write-probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	f.Close()
	return os.Remove(probe)
}

// SaveSecrets writes the secrets file with mode 0600.
func SaveSecrets(s Secrets) error {
	dir := config.Dir("agent-helper")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := secretsPath()
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := toml.NewEncoder(f).Encode(s); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	return os.Rename(tmp, path)
}

// ApplyEnv copies non-empty secret fields into os.Environ, but only
// when the corresponding variable isn't already set. The mapping
// matches Provider.EnvForward names so the existing forwarding code
// just works.
func (s Secrets) ApplyEnv() {
	apply := func(key, val string) {
		if val == "" {
			return
		}
		if _, ok := os.LookupEnv(key); ok {
			return // user's shell wins
		}
		_ = os.Setenv(key, val)
	}
	apply("CLAUDE_CODE_OAUTH_TOKEN", s.ClaudeOAuthToken)
	apply("OPENAI_API_KEY", s.OpenAIAPIKey)
}
