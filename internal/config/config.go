package config

import (
	"bytes"
	"fmt"
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

// Save encodes the struct as TOML and writes it atomically: the encoded
// bytes go to a sibling temp file first, then os.Rename moves them into
// place. A mid-encode failure or interrupt therefore can't leave the
// real config truncated.
func Save(toolName string, v any) error {
	dir := Dir(toolName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(v); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	final := Path(toolName)
	tmp, err := os.CreateTemp(dir, ".config.toml.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, final); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
