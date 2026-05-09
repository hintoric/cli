package plugins_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/hintoric/cli/internal/plugins"
	"github.com/hintoric/cli/internal/signer"
)

// TestEndToEnd builds the hint-echo test plugin, sets up a fake registry, and
// drives Runner.Run from install through dispatch. It exercises the full v1
// plugin contract end-to-end.
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short")
	}

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Join(root, "..", "..")

	// Build the test plugin into a temp dir.
	pluginDir := t.TempDir()
	binPath := filepath.Join(pluginDir, "hint-echo"+plugins.BinaryExtension())
	buildArgs := []string{"build"}
	// On darwin the internal Go linker omits LC_UUID (required by dyld on
	// macOS 26+) and produces an ad-hoc linker-signed signature that becomes
	// invalid once the binary is copied off the build path (which the
	// installer does). -s -w + external linker yields a binary that actually
	// runs after install.
	if runtime.GOOS == "darwin" {
		buildArgs = append(buildArgs, "-ldflags=-s -w -linkmode=external")
	}
	buildArgs = append(buildArgs, "-o", binPath, ".")
	build := exec.Command("go", buildArgs...)
	build.Dir = filepath.Join(repoRoot, "testdata", "hint-echo")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build hint-echo: %v\n%s", err, out)
	}
	body, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	sumHex := hex.EncodeToString(sum[:])

	// Build the manifest in memory.
	pl := &plugins.PluginList{
		SchemaVersion: plugins.SchemaVersionCurrent,
		Plugins: []plugins.Plugin{
			{
				Shortname:        "echo",
				Shortdesc:        "echo test plugin",
				Binary:           "hint-echo",
				MagicCookieValue: "h1nt-ech0-c00k1e",
				Releases: []plugins.Release{
					{Version: "0.1.0", OS: runtime.GOOS, Arch: runtime.GOARCH, Sum: sumHex},
				},
			},
		},
	}
	manifestBuf := mustEncodeTOML(t, pl)

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := signer.Sign(priv, manifestBuf)
	if err != nil {
		t.Fatal(err)
	}

	// Stand up the registry.
	mux := http.NewServeMux()
	mux.HandleFunc("/plugins.toml", func(w http.ResponseWriter, r *http.Request) { w.Write(manifestBuf) })
	mux.HandleFunc("/plugins.toml.sig", func(w http.ResponseWriter, r *http.Request) { w.Write(sig) })
	binPathURL := fmt.Sprintf("/echo/0.1.0/%s/%s/hint-echo%s", runtime.GOOS, runtime.GOARCH, plugins.BinaryExtension())
	mux.HandleFunc(binPathURL, func(w http.ResponseWriter, r *http.Request) { w.Write(body) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Wire the loader, installer, runner.
	cfgDir := t.TempDir()
	cacheDir := t.TempDir()
	loader := &plugins.ManifestLoader{
		BaseURL: srv.URL, CacheDir: cfgDir, PublicKey: &priv.PublicKey,
	}
	pl2, err := loader.Get(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	plug := plugins.FindPlugin(pl2, "echo")
	if plug == nil {
		t.Fatal("echo plugin missing from manifest")
	}
	inst := &plugins.Installer{
		BaseURL: srv.URL, ConfigDir: cfgDir, CacheDir: cacheDir, HTTPClient: srv.Client(),
	}
	runner := &plugins.Runner{ConfigDir: cfgDir, Installer: inst}

	// Happy path: hint echo hello world → exit 0
	code, err := runner.Run(context.Background(), plug, []string{"hello", "world"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}

	// Failure path: first arg "fail" → exit 7
	code, err = runner.Run(context.Background(), plug, []string{"fail"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if code != 7 {
		t.Errorf("expected exit 7, got %d", code)
	}
}

func mustEncodeTOML(t *testing.T, pl *plugins.PluginList) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(pl); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
