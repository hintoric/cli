package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledVersionAbsent(t *testing.T) {
	if v := InstalledVersion(t.TempDir(), "missing"); v != "" {
		t.Errorf("got %q, want empty", v)
	}
}

func TestInstalledVersionPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "plugins", "pipeline", "0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if v := InstalledVersion(dir, "pipeline"); v != "0.1.0" {
		t.Errorf("got %q, want 0.1.0", v)
	}
}

func TestUninstallRemovesDir(t *testing.T) {
	dir := t.TempDir()
	pluginRoot := filepath.Join(dir, "plugins", "pipeline")
	if err := os.MkdirAll(filepath.Join(pluginRoot, "0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(dir, "pipeline"); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(pluginRoot); !os.IsNotExist(err) {
		t.Fatalf("expected plugin root removed, err=%v", err)
	}
}

func TestUninstallMissingIsError(t *testing.T) {
	if err := Uninstall(t.TempDir(), "ghost"); err == nil {
		t.Fatal("expected error for missing plugin")
	}
}
