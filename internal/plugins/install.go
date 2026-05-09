package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Installer downloads, verifies, and lays out plugin binaries on disk.
type Installer struct {
	BaseURL    string
	ConfigDir  string
	CacheDir   string
	HTTPClient *http.Client
}

// Install downloads the binary for (plugin, version) matching the current
// GOOS/GOARCH, verifies its sha256 against the manifest, and atomically
// places it at BinaryPath(...). After success, all other versions of the
// plugin are removed from disk.
func (inst *Installer) Install(ctx context.Context, p *Plugin, ver string) error {
	rel := SelectExactRelease(p, ver, runtime.GOOS, runtime.GOARCH)
	if rel == nil {
		return fmt.Errorf("no release of %s@%s for %s/%s",
			p.Shortname, ver, runtime.GOOS, runtime.GOARCH)
	}

	url := fmt.Sprintf("%s/%s/%s/%s/%s/%s",
		inst.BaseURL, p.Shortname, ver, runtime.GOOS, runtime.GOARCH, p.Binary)

	body, err := inst.download(ctx, url)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if got != rel.Sum {
		return fmt.Errorf("sha256 mismatch for %s@%s: got %s, want %s",
			p.Shortname, ver, got, rel.Sum)
	}

	dstDir := InstallPath(inst.ConfigDir, p.Shortname, ver)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dstDir, err)
	}
	dst := BinaryPath(inst.ConfigDir, p.Shortname, ver, p.Binary)
	if err := atomicWriteExec(dst, body); err != nil {
		return err
	}

	// Remove other versions of this plugin.
	pluginRoot := filepath.Join(inst.ConfigDir, "plugins", p.Shortname)
	entries, err := os.ReadDir(pluginRoot)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != ver {
				_ = os.RemoveAll(filepath.Join(pluginRoot, e.Name()))
			}
		}
	}
	return nil
}

func (inst *Installer) download(ctx context.Context, url string) ([]byte, error) {
	c := inst.HTTPClient
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func atomicWriteExec(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".hint-bin-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
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
