package plugins

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ManifestLoader fetches the signed plugin manifest, verifies it, caches it,
// and serves it on subsequent calls within the TTL window.
type ManifestLoader struct {
	BaseURL   string           // e.g. https://cli.hintoric.com
	CacheDir  string           // e.g. ~/.config/hint
	PublicKey *ecdsa.PublicKey // embedded into host binary (ECDSA P-256)
	TTL       time.Duration    // default 1h; 0 means always refresh
	Now       func() time.Time // injected for tests; nil → time.Now
	HTTP      *http.Client     // optional override
}

const (
	manifestFile    = "manifest.toml"
	manifestSigFile = "manifest.toml.sig"
	fetchedAtFile   = "manifest.fetched_at"
	manifestRemote  = "plugins.toml"
	signatureRemote = "plugins.toml.sig"
)

// Get returns the current manifest, fetching from the network if the cache is
// stale or absent. force=true bypasses the TTL check. On network failure with a
// usable cache, falls back to cache and writes a warning to stderr.
func (l *ManifestLoader) Get(ctx context.Context, force bool) (*PluginList, error) {
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}

	if !force && l.cacheFresh(now()) {
		return l.loadFromCache()
	}

	pl, err := l.fetchAndCache(ctx)
	if err == nil {
		return pl, nil
	}

	// Fall back to cache on network errors.
	if cached, cacheErr := l.loadFromCache(); cacheErr == nil {
		fmt.Fprintf(os.Stderr, "warning: manifest fetch failed (%v); using cached copy\n", err)
		return cached, nil
	}
	return nil, err
}

func (l *ManifestLoader) cacheFresh(now time.Time) bool {
	if l.TTL <= 0 {
		return false
	}
	data, err := os.ReadFile(filepath.Join(l.CacheDir, fetchedAtFile))
	if err != nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return false
	}
	return now.Sub(t) < l.TTL
}

func (l *ManifestLoader) loadFromCache() (*PluginList, error) {
	body, err := os.ReadFile(filepath.Join(l.CacheDir, manifestFile))
	if err != nil {
		return nil, fmt.Errorf("read cached manifest: %w", err)
	}
	sig, err := os.ReadFile(filepath.Join(l.CacheDir, manifestSigFile))
	if err != nil {
		return nil, fmt.Errorf("read cached signature: %w", err)
	}
	if err := VerifyManifestSignature(l.PublicKey, body, sig); err != nil {
		return nil, err
	}
	pl, err := ParseManifest(body)
	if err != nil {
		return nil, err
	}
	if err := ValidateManifest(pl); err != nil {
		return nil, err
	}
	return pl, nil
}

func (l *ManifestLoader) fetchAndCache(ctx context.Context) (*PluginList, error) {
	body, err := l.fetch(ctx, manifestRemote)
	if err != nil {
		return nil, err
	}
	sig, err := l.fetch(ctx, signatureRemote)
	if err != nil {
		return nil, err
	}
	if err := VerifyManifestSignature(l.PublicKey, body, sig); err != nil {
		return nil, err
	}
	pl, err := ParseManifest(body)
	if err != nil {
		return nil, err
	}
	if err := ValidateManifest(pl); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(l.CacheDir, 0o755); err != nil {
		return nil, err
	}
	if err := atomicWrite(filepath.Join(l.CacheDir, manifestFile), body, 0o644); err != nil {
		return nil, err
	}
	if err := atomicWrite(filepath.Join(l.CacheDir, manifestSigFile), sig, 0o644); err != nil {
		return nil, err
	}
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}
	if err := atomicWrite(filepath.Join(l.CacheDir, fetchedAtFile),
		[]byte(now().UTC().Format(time.RFC3339)), 0o644); err != nil {
		return nil, err
	}
	return pl, nil
}

func (l *ManifestLoader) fetch(ctx context.Context, name string) ([]byte, error) {
	url := l.BaseURL + "/" + name
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c := l.HTTP
	if c == nil {
		c = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	return body, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".hint-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
