package plugins

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hintoric/cli/internal/signer"
)

func newTestRegistry(t *testing.T, body, sig []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/plugins.toml", func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	})
	mux.HandleFunc("/plugins.toml.sig", func(w http.ResponseWriter, r *http.Request) {
		w.Write(sig)
	})
	return httptest.NewServer(mux)
}

func TestFetchAndVerifyManifest(t *testing.T) {
	priv := newTestKey(t)
	body := []byte(sampleManifest)
	sig, err := signer.Sign(priv, body)
	if err != nil {
		t.Fatal(err)
	}

	srv := newTestRegistry(t, body, sig)
	defer srv.Close()

	cacheDir := t.TempDir()
	loader := &ManifestLoader{
		BaseURL:   srv.URL,
		CacheDir:  cacheDir,
		PublicKey: &priv.PublicKey,
		TTL:       time.Hour,
		Now:       time.Now,
	}

	pl, err := loader.Get(context.Background(), false)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(pl.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(pl.Plugins))
	}

	for _, name := range []string{"manifest.toml", "manifest.toml.sig", "manifest.fetched_at"} {
		if _, err := readFile(filepath.Join(cacheDir, name)); err != nil {
			t.Errorf("expected cache file %s, got %v", name, err)
		}
	}
}

func TestCacheHitWithinTTL(t *testing.T) {
	priv := newTestKey(t)
	body := []byte(sampleManifest)
	sig, err := signer.Sign(priv, body)
	if err != nil {
		t.Fatal(err)
	}

	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/plugins.toml", func(w http.ResponseWriter, r *http.Request) { hits++; w.Write(body) })
	mux.HandleFunc("/plugins.toml.sig", func(w http.ResponseWriter, r *http.Request) { hits++; w.Write(sig) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	loader := &ManifestLoader{
		BaseURL:   srv.URL,
		CacheDir:  t.TempDir(),
		PublicKey: &priv.PublicKey,
		TTL:       time.Hour,
		Now:       time.Now,
	}
	if _, err := loader.Get(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	hitsAfterFirst := hits
	if _, err := loader.Get(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if hits != hitsAfterFirst {
		t.Fatalf("expected cache hit, but server got %d more requests", hits-hitsAfterFirst)
	}
}

func TestForceRefresh(t *testing.T) {
	priv := newTestKey(t)
	body := []byte(sampleManifest)
	sig, err := signer.Sign(priv, body)
	if err != nil {
		t.Fatal(err)
	}
	srv := newTestRegistry(t, body, sig)
	defer srv.Close()

	loader := &ManifestLoader{
		BaseURL:   srv.URL,
		CacheDir:  t.TempDir(),
		PublicKey: &priv.PublicKey,
		TTL:       time.Hour,
		Now:       time.Now,
	}
	if _, err := loader.Get(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := loader.Get(context.Background(), true); err != nil {
		t.Fatalf("force refresh: %v", err)
	}
}

func TestOfflineFallback(t *testing.T) {
	priv := newTestKey(t)
	body := []byte(sampleManifest)
	sig, err := signer.Sign(priv, body)
	if err != nil {
		t.Fatal(err)
	}
	srv := newTestRegistry(t, body, sig)

	loader := &ManifestLoader{
		BaseURL:   srv.URL,
		CacheDir:  t.TempDir(),
		PublicKey: &priv.PublicKey,
		TTL:       time.Hour,
		Now:       time.Now,
	}
	if _, err := loader.Get(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	srv.Close()
	// Force refresh past TTL but with server down → must fall back to cache.
	loader.TTL = 0
	pl, err := loader.Get(context.Background(), false)
	if err != nil {
		t.Fatalf("expected offline fallback to succeed, got %v", err)
	}
	if len(pl.Plugins) != 1 {
		t.Fatalf("expected 1 plugin from cache, got %d", len(pl.Plugins))
	}
}

func TestRejectsBadSignatureOnFetch(t *testing.T) {
	priv := newTestKey(t)
	body := []byte(sampleManifest)
	// Construct a syntactically valid but mathematically invalid signature
	// by signing a different message with a different key.
	other := newTestKey(t)
	otherSig, err := signer.Sign(other, []byte("not the body"))
	if err != nil {
		t.Fatal(err)
	}

	srv := newTestRegistry(t, body, otherSig)
	defer srv.Close()

	loader := &ManifestLoader{
		BaseURL:   srv.URL,
		CacheDir:  t.TempDir(),
		PublicKey: &priv.PublicKey,
		TTL:       time.Hour,
		Now:       time.Now,
	}
	if _, err := loader.Get(context.Background(), false); err == nil {
		t.Fatal("expected signature failure")
	}
}

func readFile(p string) ([]byte, error) { return os.ReadFile(p) }
