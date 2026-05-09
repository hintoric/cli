package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallDownloadsAndVerifies(t *testing.T) {
	binBody := []byte("#!/usr/bin/env echo\nfake plugin binary")
	sum := sha256.Sum256(binBody)

	mux := http.NewServeMux()
	mux.HandleFunc("/pipeline/0.1.0/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(binBody)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfgDir := t.TempDir()
	cacheDir := t.TempDir()

	p := &Plugin{
		Shortname: "pipeline",
		Binary:    "hint-pipeline",
		Releases: []Release{
			{Version: "0.1.0", OS: "darwin", Arch: "arm64", Sum: hex.EncodeToString(sum[:])},
			{Version: "0.1.0", OS: "linux", Arch: "amd64", Sum: hex.EncodeToString(sum[:])},
			{Version: "0.1.0", OS: "darwin", Arch: "amd64", Sum: hex.EncodeToString(sum[:])},
			{Version: "0.1.0", OS: "linux", Arch: "arm64", Sum: hex.EncodeToString(sum[:])},
			{Version: "0.1.0", OS: "windows", Arch: "amd64", Sum: hex.EncodeToString(sum[:])},
		},
	}

	inst := &Installer{
		BaseURL:    srv.URL,
		ConfigDir:  cfgDir,
		CacheDir:   cacheDir,
		HTTPClient: srv.Client(),
	}

	if err := inst.Install(context.Background(), p, "0.1.0"); err != nil {
		t.Fatalf("install: %v", err)
	}

	binPath := BinaryPath(cfgDir, "pipeline", "0.1.0", "hint-pipeline")
	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(got) != string(binBody) {
		t.Fatal("binary content mismatch")
	}
	info, _ := os.Stat(binPath)
	if info.Mode()&0o100 == 0 {
		t.Errorf("binary not executable: mode %v", info.Mode())
	}
}

func TestInstallRejectsTamperedBinary(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pipeline/0.1.0/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("tampered content"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := &Plugin{
		Shortname: "pipeline", Binary: "hint-pipeline",
		Releases: allPlatforms("0.1.0", "0000000000000000000000000000000000000000000000000000000000000000"),
	}
	inst := &Installer{
		BaseURL: srv.URL, ConfigDir: t.TempDir(), CacheDir: t.TempDir(),
		HTTPClient: srv.Client(),
	}
	err := inst.Install(context.Background(), p, "0.1.0")
	if err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
}

func TestInstallCleansUpOldVersions(t *testing.T) {
	binBody := []byte("v2 binary")
	sum := sha256.Sum256(binBody)

	mux := http.NewServeMux()
	mux.HandleFunc("/pipeline/0.2.0/", func(w http.ResponseWriter, r *http.Request) { w.Write(binBody) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfgDir := t.TempDir()
	// Pre-populate an old version dir.
	oldDir := InstallPath(cfgDir, "pipeline", "0.1.0")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "hint-pipeline"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := &Plugin{
		Shortname: "pipeline", Binary: "hint-pipeline",
		Releases: allPlatforms("0.2.0", hex.EncodeToString(sum[:])),
	}
	inst := &Installer{
		BaseURL: srv.URL, ConfigDir: cfgDir, CacheDir: t.TempDir(),
		HTTPClient: srv.Client(),
	}
	if err := inst.Install(context.Background(), p, "0.2.0"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("expected old version dir removed, got err=%v", err)
	}
}

// helper used by multiple tests
func allPlatforms(ver, sum string) []Release {
	return []Release{
		{Version: ver, OS: "darwin", Arch: "arm64", Sum: sum},
		{Version: ver, OS: "darwin", Arch: "amd64", Sum: sum},
		{Version: ver, OS: "linux", Arch: "arm64", Sum: sum},
		{Version: ver, OS: "linux", Arch: "amd64", Sum: sum},
		{Version: ver, OS: "windows", Arch: "amd64", Sum: sum},
	}
}
