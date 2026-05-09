package cmd

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/config"
	"github.com/hintoric/cli/internal/lock"
	"github.com/hintoric/cli/internal/plugins"
	"github.com/hintoric/cli/internal/signer"
)

// pluginDeps bundles the shared state every `hint plugin` subcommand needs.
type pluginDeps struct {
	pubKey    *ecdsa.PublicKey
	configDir string
	cacheDir  string
	cfgPath   string
	cfg       *config.Config
	loader    *plugins.ManifestLoader
	installer *plugins.Installer
	runner    *plugins.Runner
	lock      *lock.Lock
}

// newPluginDeps parses the embedded public key, loads config, and constructs
// the manifest loader, installer, runner, and lock used by plugin subcommands.
// Honors HINT_MANIFEST_TTL (a Go duration string) to override the 1h default.
func newPluginDeps(pubKeyPEM []byte) (*pluginDeps, error) {
	pub, err := signer.ParsePublicKeyPEM(pubKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse embedded pubkey: %w", err)
	}
	cfgDir := config.ConfigDir()
	cfgPath := config.ConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	ttl := time.Hour
	if v := os.Getenv("HINT_MANIFEST_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ttl = d
		}
	}

	deps := &pluginDeps{
		pubKey:    pub,
		configDir: cfgDir,
		cacheDir:  config.CacheDir(),
		cfgPath:   cfgPath,
		cfg:       cfg,
		loader: &plugins.ManifestLoader{
			BaseURL:   cfg.ManifestURL,
			CacheDir:  cfgDir,
			PublicKey: pub,
			TTL:       ttl,
		},
		installer: &plugins.Installer{
			BaseURL:    cfg.ManifestURL,
			ConfigDir:  cfgDir,
			CacheDir:   config.CacheDir(),
			HTTPClient: &http.Client{Timeout: 5 * time.Minute},
		},
		lock: lock.New(filepath.Join(cfgDir, ".lock")),
	}
	deps.runner = &plugins.Runner{ConfigDir: cfgDir, Installer: deps.installer}
	return deps, nil
}

// pluginParent builds the `hint plugin` cobra parent command and registers its
// subcommands: install, uninstall, update, and list.
func pluginParent(pubKeyPEM []byte) *cobra.Command {
	parent := &cobra.Command{
		Use:   "plugin",
		Short: "Manage hint plugins",
	}
	parent.AddCommand(pluginInstallCommand(pubKeyPEM))
	parent.AddCommand(pluginUninstallCommand(pubKeyPEM))
	parent.AddCommand(pluginUpdateCommand(pubKeyPEM))
	parent.AddCommand(pluginListCommand(pubKeyPEM))
	return parent
}

// addInstalledIfMissing ensures shortname is recorded in cfg.InstalledPlugins.
func addInstalledIfMissing(cfg *config.Config, shortname string) {
	for _, n := range cfg.InstalledPlugins {
		if n == shortname {
			return
		}
	}
	cfg.InstalledPlugins = append(cfg.InstalledPlugins, shortname)
}

// removeInstalled drops shortname from cfg.InstalledPlugins if present.
func removeInstalled(cfg *config.Config, shortname string) {
	out := cfg.InstalledPlugins[:0]
	for _, n := range cfg.InstalledPlugins {
		if n != shortname {
			out = append(out, n)
		}
	}
	cfg.InstalledPlugins = out
}

// withLockedDeps builds plugin deps, acquires the file lock with a 30s
// timeout, runs fn, and always releases the lock on return.
func withLockedDeps(pubKeyPEM []byte, fn func(context.Context, *pluginDeps) error) error {
	deps, err := newPluginDeps(pubKeyPEM)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := deps.lock.Acquire(ctx); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer deps.lock.Release()
	return fn(ctx, deps)
}
