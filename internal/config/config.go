// Package config loads and saves ~/.config/hint/config.toml and reports
// the canonical hint config directory respecting HINT_CONFIG_DIR and XDG.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// DefaultManifestURL is the production registry root.
const DefaultManifestURL = "https://cdn.hintoric.com/cli"

// Config maps to the on-disk config.toml.
type Config struct {
	ManifestURL      string                  `toml:"manifest_url"`
	InstalledPlugins []string                `toml:"installed_plugins"`
	Plugin           map[string]PluginConfig `toml:"plugin"`
}

// PluginConfig is per-plugin user-tunable state.
type PluginConfig struct {
	PinnedVersion string `toml:"pinned_version"`
}

// ConfigDir returns the absolute path of the hint config directory.
// HINT_CONFIG_DIR > $XDG_CONFIG_HOME/hint > $HOME/.config/hint.
func ConfigDir() string {
	if v := os.Getenv("HINT_CONFIG_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "hint")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Last-resort fallback. The caller will fail later if this isn't writable.
		home = "."
	}
	return filepath.Join(home, ".config", "hint")
}

// CacheDir returns the absolute path of the hint cache directory.
// $XDG_CACHE_HOME/hint > $HOME/.cache/hint.
func CacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "hint")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cache", "hint")
}

// ConfigPath returns the absolute path of config.toml.
func ConfigPath() string { return filepath.Join(ConfigDir(), "config.toml") }

// Load reads config from path, returning defaults if the file is absent.
func Load(path string) (*Config, error) {
	cfg := &Config{
		ManifestURL: DefaultManifestURL,
		Plugin:      map[string]PluginConfig{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.ManifestURL == "" {
		cfg.ManifestURL = DefaultManifestURL
	}
	if cfg.Plugin == nil {
		cfg.Plugin = map[string]PluginConfig{}
	}
	return cfg, nil
}

// Save writes the config atomically to path.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
