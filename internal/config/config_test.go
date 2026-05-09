package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigDirOverride(t *testing.T) {
	t.Setenv("HINT_CONFIG_DIR", "/tmp/custom")
	got := ConfigDir()
	if got != "/tmp/custom" {
		t.Fatalf("got %q, want /tmp/custom", got)
	}
}

func TestConfigDirXDG(t *testing.T) {
	t.Setenv("HINT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got := ConfigDir()
	if got != "/tmp/xdg/hint" {
		t.Fatalf("got %q, want /tmp/xdg/hint", got)
	}
}

func TestLoadDefaultsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ManifestURL != DefaultManifestURL {
		t.Errorf("ManifestURL got %q, want %q", cfg.ManifestURL, DefaultManifestURL)
	}
	if len(cfg.InstalledPlugins) != 0 {
		t.Errorf("InstalledPlugins should be empty, got %v", cfg.InstalledPlugins)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := &Config{
		ManifestURL:      "https://staging.cli.hintoric.com",
		InstalledPlugins: []string{"pipeline", "aws"},
		Plugin: map[string]PluginConfig{
			"pipeline": {PinnedVersion: "0.1.0"},
		},
	}
	if err := original.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(loaded, original) {
		t.Fatalf("round-trip mismatch:\nwant %+v\ngot  %+v", original, loaded)
	}
}
