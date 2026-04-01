package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Dir returns the config directory for a tool: ~/.config/{toolName}/
func Dir(toolName string) string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, toolName)
}

// Path returns the config file path: ~/.config/{toolName}/config.toml
func Path(toolName string) string {
	return filepath.Join(Dir(toolName), "config.toml")
}

// Exists returns true if the config file exists.
func Exists(toolName string) bool {
	_, err := os.Stat(Path(toolName))
	return err == nil
}

// Load decodes the TOML config file into the provided struct.
func Load(toolName string, v any) error {
	_, err := toml.DecodeFile(Path(toolName), v)
	return err
}

// Save encodes the struct as TOML and writes it to the config file.
func Save(toolName string, v any) error {
	dir := Dir(toolName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(Path(toolName))
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(v)
}
