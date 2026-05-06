package cmd

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const projectConfigName = ".agent-helper.toml"

// findProjectConfig walks up from cwd to find a `.agent-helper.toml` file.
// Returns "" if none.
func findProjectConfig(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(abs, projectConfigName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}

// loadEffectiveConfig returns the global config overlaid with any
// `.agent-helper.toml` discovered above `cwd`. Project values override
// global values when set (non-zero). Pack lists from project replace
// global default_packs (set to []string{} to clear).
func loadEffectiveConfig(cwd string) Config {
	cfg := loadConfig()
	path := findProjectConfig(cwd)
	if path == "" {
		return cfg
	}
	var proj Config
	if _, err := toml.DecodeFile(path, &proj); err != nil {
		return cfg
	}
	if proj.Defaults.Agent != "" {
		cfg.Defaults.Agent = proj.Defaults.Agent
	}
	if proj.Defaults.Isolated != "" {
		cfg.Defaults.Isolated = proj.Defaults.Isolated
	}
	if proj.Local.URL != "" {
		cfg.Local.URL = proj.Local.URL
	}
	if proj.Local.DefaultModel != "" {
		cfg.Local.DefaultModel = proj.Local.DefaultModel
	}
	if proj.Orb.Distro != "" {
		cfg.Orb.Distro = proj.Orb.Distro
	}
	if proj.Orb.Arch != "" {
		cfg.Orb.Arch = proj.Orb.Arch
	}
	if proj.Orb.SharedIdleDays != 0 {
		cfg.Orb.SharedIdleDays = proj.Orb.SharedIdleDays
	}
	if len(proj.Orb.MountRoots) > 0 {
		cfg.Orb.MountRoots = proj.Orb.MountRoots
	}
	if proj.Orb.DefaultPacks != nil {
		// Empty slice resets to no defaults; nil keeps the global list.
		cfg.Orb.DefaultPacks = proj.Orb.DefaultPacks
	}
	return cfg
}
