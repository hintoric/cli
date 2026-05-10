package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
	"golang.org/x/term"

	"github.com/hintoric/cli/internal/plugins/proto"

	hcversion "github.com/hashicorp/go-version"
)

// Runner connects the host to a plugin process via hcplugin and dispatches
// the user's command.
type Runner struct {
	ConfigDir string
	Installer *Installer // used for lazy install when not present locally
	EnvAllow  []string   // env keys forwarded to plugin (default if nil)

	// Stdout / Stderr are where plugin Print() output is written. Defaults
	// to os.Stdout / os.Stderr when nil; tests inject buffers.
	Stdout io.Writer
	Stderr io.Writer
}

// defaultEnvAllow lists the env keys the host forwards to plugins. Anything
// else is stripped to keep secrets from leaking accidentally. Prefixes that
// match a whole family (LC_*, XDG_*) are listed once and matched as prefixes.
var defaultEnvAllow = []string{
	"PATH", "HOME", "USER", "SHELL", "PWD",
	"TERM", "COLORTERM", "NO_COLOR", "FORCE_COLOR",
	"LANG", "LANGUAGE",
	"TMPDIR", "TEMP", "TMP",
	// Network / proxy
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",
	"http_proxy", "https_proxy", "no_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR",
	// CI signal so plugins can reduce output / disable interactivity
	"CI", "GITHUB_ACTIONS",
}

// defaultEnvAllowPrefixes — env keys whose value+anything-after pass through.
// LC_* covers the locale family, XDG_* covers freedesktop user paths.
var defaultEnvAllowPrefixes = []string{"LC_", "XDG_", "HINT_"}

// Run resolves an installed version (installing if needed), starts the plugin
// via hcplugin, calls RunCommand, and returns the plugin's exit code.
//
// Dev mode (skips manifest lookup + checksum verification) requires BOTH:
//   - HINT_DEV_MODE=1
//   - either HINT_PLUGINS_PATH set, or a "local.dev" version dir on disk
//
// Both gates are deliberate: a stale HINT_PLUGINS_PATH alone (e.g. left in
// a developer's shell rc) is not enough to silently bypass verification.
// When dev mode triggers, a clear warning is printed to stderr.
func (r *Runner) Run(ctx context.Context, p *Plugin, args []string) (int, error) {
	ver, devMode, err := r.resolveVersion(ctx, p)
	if err != nil {
		return 1, err
	}
	if devMode {
		fmt.Fprintf(os.Stderr,
			"warning: %s running in DEV MODE — checksum verification skipped\n",
			p.Shortname)
	}

	var sumBytes []byte
	if !devMode {
		rel := SelectExactRelease(p, ver, runtime.GOOS, runtime.GOARCH)
		if rel == nil {
			return 1, fmt.Errorf("no release of %s@%s for %s/%s", p.Shortname, ver, runtime.GOOS, runtime.GOARCH)
		}
		sumBytes, err = hex.DecodeString(rel.Sum)
		if err != nil {
			return 1, fmt.Errorf("decode sum: %w", err)
		}
	}

	binPath := r.resolveBinaryPath(p, ver, devMode)

	cmd := exec.Command(binPath)
	cmd.Env = r.filteredEnv()
	// Belt-and-braces: hcplugin's ClientConfig.SkipHostEnv flag (set below)
	// is what actually prevents the host environment from being appended to
	// cmd.Env. Without it, the value we just set here is silently extended
	// with os.Environ() inside hcplugin.NewClient — the allowlist would be
	// bypassed and any AWS_*/GITHUB_*/etc. secrets in the host env would
	// reach the plugin process.

	// hcplugin captures the plugin's stderr and routes it through this
	// logger. Plugin stderr writes (fmt.Fprintln on os.Stderr) parse as
	// INFO-level lines; a stricter level silently drops them. Info also
	// keeps hcplugin's own internal chatter (mostly Debug+) out of the
	// user's terminal.
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "hint.plugin." + p.Shortname,
		Output: os.Stderr,
		Level:  hclog.Info,
	})
	clientConfig := &hcplugin.ClientConfig{
		HandshakeConfig:  HandshakeConfig(p.Shortname, p.MagicCookieValue),
		VersionedPlugins: PluginSet(nil),
		Cmd:              cmd,
		SyncStdout:       os.Stdout,
		SyncStderr:       os.Stderr,
		Logger:           logger,
		Managed:          true,
		AllowedProtocols: []hcplugin.Protocol{hcplugin.ProtocolGRPC},
		// Without this hcplugin appends os.Environ() to cmd.Env, so the
		// filteredEnv() allowlist above would be a no-op and the plugin
		// would inherit every env var the host process has (AWS creds,
		// GITHUB_TOKEN, ...). The cookie env-var is appended by hcplugin
		// itself regardless of this flag.
		SkipHostEnv: true,
	}
	// Skip checksum entirely in dev mode — the binary is whatever the
	// developer just built, no manifest entry to verify against.
	if !devMode {
		clientConfig.SecureConfig = &hcplugin.SecureConfig{
			Checksum: sumBytes,
			Hash:     sha256.New(),
		}
	}
	client := hcplugin.NewClient(clientConfig)
	// Without this the plugin subprocess survives after Run returns on the
	// error path. Managed: true above only guarantees cleanup via the
	// package-global hcplugin.CleanupClients, which library consumers don't
	// normally call. An explicit Kill is the lifecycle-tied teardown.
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		return 1, fmt.Errorf("connect plugin: %w", err)
	}
	raw, err := rpcClient.Dispense("main")
	if err != nil {
		return 1, fmt.Errorf("dispense main: %w", err)
	}
	disp, ok := raw.(*clientV1)
	if !ok {
		return 1, fmt.Errorf("unexpected dispenser type %T", raw)
	}

	stdout := io.Writer(os.Stdout)
	stderr := io.Writer(os.Stderr)
	if r.Stdout != nil {
		stdout = r.Stdout
	}
	if r.Stderr != nil {
		stderr = r.Stderr
	}

	info := buildAdditionalInfo()
	code, err := disp.RunCommand(ctx, info, args, stdout, stderr)
	if err != nil {
		// disp.RunCommand returned -1 on RPC failure; pass it through
		// rather than collapsing to 1 so callers can distinguish
		// transport/protocol failures from plugin-reported non-zero exits.
		return int(code), fmt.Errorf("plugin run: %w", err)
	}
	return int(code), nil
}

// resolveBinaryPath returns the on-disk binary location, applying the
// HINT_PLUGINS_PATH env override if set.
func (r *Runner) resolveBinaryPath(p *Plugin, ver string, devMode bool) string {
	if root := os.Getenv("HINT_PLUGINS_PATH"); root != "" && devMode {
		return filepath.Join(root, p.Shortname, p.Binary+BinaryExtension())
	}
	return BinaryPath(r.ConfigDir, p.Shortname, ver, p.Binary)
}

// resolveVersion returns the version to use, plus a flag indicating dev mode
// is active. Dev mode skips manifest lookup + checksum verification and
// requires BOTH HINT_DEV_MODE=1 AND a dev path/version (HINT_PLUGINS_PATH or
// a local.dev directory on disk). Either gate alone falls back to the
// normal manifest-verified flow.
func (r *Runner) resolveVersion(ctx context.Context, p *Plugin) (string, bool, error) {
	devGated := os.Getenv("HINT_DEV_MODE") == "1"
	envPath := os.Getenv("HINT_PLUGINS_PATH")

	// HINT_PLUGINS_PATH dev mode: skip the on-disk version lookup entirely.
	if devGated && envPath != "" {
		return LocalDevelopmentVersion, true, nil
	}

	if v := InstalledVersion(r.ConfigDir, p.Shortname); v != "" {
		// local.dev directory dev mode: only honoured when explicitly gated.
		if v == LocalDevelopmentVersion {
			if devGated {
				return v, true, nil
			}
			// local.dev exists but the gate isn't set — refuse rather than
			// silently downgrading. Operator must opt in.
			return "", false, fmt.Errorf("plugin %q has a local.dev install but HINT_DEV_MODE=1 is not set", p.Shortname)
		}
		return v, false, nil
	}
	rel := SelectLatestRelease(p, runtime.GOOS, runtime.GOARCH)
	if rel == nil {
		return "", false, fmt.Errorf("no release of %s for %s/%s", p.Shortname, runtime.GOOS, runtime.GOARCH)
	}
	if err := r.Installer.Install(ctx, p, rel.Version); err != nil {
		return "", false, err
	}
	return rel.Version, false, nil
}

// filteredEnv returns the subset of the host environment forwarded to plugins.
// hcplugin appends the magic cookie env var itself, so we don't add it here.
func (r *Runner) filteredEnv() []string {
	allow := r.EnvAllow
	if allow == nil {
		allow = defaultEnvAllow
	}
	keys := map[string]bool{}
	for _, k := range allow {
		keys[k] = true
	}
	out := []string{}
	for _, e := range os.Environ() {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			continue
		}
		k := e[:eq]
		if keys[k] {
			out = append(out, e)
			continue
		}
		for _, prefix := range defaultEnvAllowPrefixes {
			if strings.HasPrefix(k, prefix) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

func buildAdditionalInfo() *proto.AdditionalInfo {
	info := &proto.AdditionalInfo{
		TerminalState: &proto.IsTerminal{
			Stdin:  term.IsTerminal(int(os.Stdin.Fd())),
			Stdout: term.IsTerminal(int(os.Stdout.Fd())),
			Stderr: term.IsTerminal(int(os.Stderr.Fd())),
		},
		Env: map[string]string{},
	}
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		info.Dims = &proto.TerminalDimensions{Width: uint32(w), Height: uint32(h)}
	}
	return info
}

// InstalledVersion returns the highest semver version directory under
// <configDir>/plugins/<shortname>/, or empty if no installation exists.
// Versions that don't parse as semver are skipped (with one exception:
// LocalDevelopmentVersion always wins if present, mirroring Stripe's
// dev-build escape hatch).
func InstalledVersion(configDir, shortname string) string {
	root := filepath.Join(configDir, "plugins", shortname)
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}

	var (
		bestName string
		bestVer  *hcversion.Version
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == LocalDevelopmentVersion {
			return name
		}
		v, err := hcversion.NewVersion(name)
		if err != nil {
			continue
		}
		if bestVer == nil || v.GreaterThan(bestVer) {
			bestName = name
			bestVer = v
		}
	}
	return bestName
}

// LocalDevelopmentVersion is a magic version string that disables checksum
// verification when paired with HINT_DEV_MODE=1. Use it to iterate on a
// plugin locally without going through publish + manifest re-sign on every
// change. Two ways to activate:
//
//	# Option 1: env-rooted plugin tree (Stripe-style)
//	HINT_DEV_MODE=1 HINT_PLUGINS_PATH=/abs/path hint <plugin> ...
//
//	# Option 2: local.dev install dir
//	mkdir -p ~/.config/hint/plugins/<plugin>/local.dev
//	cp ./hint-<plugin> ~/.config/hint/plugins/<plugin>/local.dev/
//	HINT_DEV_MODE=1 hint <plugin> ...
//
// Without HINT_DEV_MODE=1 set, both paths are refused — a stale env var
// or stray local.dev dir cannot silently bypass checksum verification.
const LocalDevelopmentVersion = "local.dev"
