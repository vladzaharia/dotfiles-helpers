package cmd

import (
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
)

// Config is the on-disk schema for ~/.config/agent-helper/config.toml.
type Config struct {
	Defaults  DefaultsConfig            `toml:"defaults"`
	Local     LocalConfig               `toml:"local"`
	Providers map[string]ProviderConfig `toml:"providers"`
	Orb       OrbConfig                 `toml:"orb"`
	Sync      SyncConfig                `toml:"sync"`
}

// DefaultsConfig captures default behavior when flags are unset.
// Per-project values can override these in `.agent-helper.toml`.
type DefaultsConfig struct {
	Agent    string `toml:"agent"`    // "claude" | "codex"
	Isolated string `toml:"isolated"` // "none" | "shared" | "full"
}

// LocalConfig describes the host-side LM Studio binding for `--local`.
type LocalConfig struct {
	Provider     string         `toml:"provider"`
	URL          string         `toml:"url"`
	DefaultModel string         `toml:"default_model"` // empty = auto-pick (Phase 7)
	Models       LocalModelTier `toml:"models"`
}

// LocalModelTier maps Anthropic-family tiers to local LM Studio model ids.
// Empty fields mean "no preference / fall back to DefaultModel".
type LocalModelTier struct {
	Haiku  string `toml:"haiku"`
	Sonnet string `toml:"sonnet"`
	Opus   string `toml:"opus"`
}

// ProviderConfig is per-provider overrides (e.g. cached real-binary path
// after shadow CLIs are installed in Phase 8).
type ProviderConfig struct {
	BinaryPath string   `toml:"binary_path"`
	ExtraArgs  []string `toml:"extra_args"`
}

// OrbConfig drives OrbStack VM creation.
type OrbConfig struct {
	Distro            string   `toml:"distro"`
	Arch              string   `toml:"arch"`
	SharedIdleDays    int      `toml:"shared_idle_days"`
	SSHControlPersist string   `toml:"ssh_control_persist"`
	MountRoots        []string `toml:"mount_roots"`
	DefaultPacks      []string `toml:"default_packs"`
}

// SyncConfig controls CWD ↔ VM sync behavior (Phase 4+).
type SyncConfig struct {
	Excludes        []string `toml:"excludes"`
	ReplaceDefaults bool     `toml:"replace_defaults"`
}

func loadConfig() Config {
	cfg := Config{
		Defaults: DefaultsConfig{
			Agent:    "claude",
			Isolated: "full", // dedicated per-project VM, ephemeral (use --keep to persist)
		},
		Local: LocalConfig{
			Provider: "lmstudio",
			URL:      "http://127.0.0.1:1234",
		},
		Orb: OrbConfig{
			Distro:            "ubuntu",
			SharedIdleDays:    7,
			SSHControlPersist: "60",
			MountRoots:        []string{"~/Repos"},
			DefaultPacks:      []string{"nodejs", "python"},
		},
	}
	_ = config.Load("agent-helper", &cfg)
	if cfg.Defaults.Agent == "" {
		cfg.Defaults.Agent = "claude"
	}
	return cfg
}
