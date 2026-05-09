package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	hclog "github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
	"golang.org/x/term"

	"github.com/hintoric/cli/internal/plugins/proto"
)

// Runner connects the host to a plugin process via hcplugin and dispatches
// the user's command.
type Runner struct {
	ConfigDir string
	Installer *Installer // used for lazy install when not present locally
	EnvAllow  []string   // env keys forwarded to plugin (default if nil)
}

// defaultEnvAllow lists the env vars the host forwards to plugins. Anything
// else is stripped to keep secrets from leaking accidentally.
var defaultEnvAllow = []string{
	"PATH", "HOME", "TERM", "LANG", "NO_COLOR",
}

// Run resolves an installed version (installing if needed), starts the plugin
// via hcplugin, calls RunCommand, and returns the plugin's exit code.
func (r *Runner) Run(ctx context.Context, p *Plugin, args []string) (int, error) {
	ver, err := r.resolveVersion(ctx, p)
	if err != nil {
		return 1, err
	}

	rel := SelectExactRelease(p, ver, runtime.GOOS, runtime.GOARCH)
	if rel == nil {
		return 1, fmt.Errorf("no release of %s@%s for %s/%s", p.Shortname, ver, runtime.GOOS, runtime.GOARCH)
	}
	sumBytes, err := hex.DecodeString(rel.Sum)
	if err != nil {
		return 1, fmt.Errorf("decode sum: %w", err)
	}

	binPath := BinaryPath(r.ConfigDir, p.Shortname, ver, p.Binary)

	cmd := exec.Command(binPath)
	cmd.Env = r.filteredEnv()

	logger := hclog.New(&hclog.LoggerOptions{Name: "hint.plugin." + p.Shortname, Level: hclog.Error})
	client := hcplugin.NewClient(&hcplugin.ClientConfig{
		HandshakeConfig:  HandshakeConfig(p.Shortname, p.MagicCookieValue),
		VersionedPlugins: PluginSet(nil),
		Cmd:              cmd,
		SyncStdout:       os.Stdout,
		SyncStderr:       os.Stderr,
		Logger:           logger,
		Managed:          true,
		AllowedProtocols: []hcplugin.Protocol{hcplugin.ProtocolGRPC},
		SecureConfig: &hcplugin.SecureConfig{
			Checksum: sumBytes,
			Hash:     sha256.New(),
		},
	})

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

	info := buildAdditionalInfo()
	code, err := disp.RunCommand(info, args)
	if err != nil {
		return 1, fmt.Errorf("plugin run: %w", err)
	}
	return int(code), nil
}

// resolveVersion returns the on-disk version, lazy-installing latest if absent.
func (r *Runner) resolveVersion(ctx context.Context, p *Plugin) (string, error) {
	if v := InstalledVersion(r.ConfigDir, p.Shortname); v != "" {
		return v, nil
	}
	rel := SelectLatestRelease(p, runtime.GOOS, runtime.GOARCH)
	if rel == nil {
		return "", fmt.Errorf("no release of %s for %s/%s", p.Shortname, runtime.GOOS, runtime.GOARCH)
	}
	if err := r.Installer.Install(ctx, p, rel.Version); err != nil {
		return "", err
	}
	return rel.Version, nil
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
		for k := range keys {
			if len(e) > len(k) && e[:len(k)] == k && e[len(k)] == '=' {
				out = append(out, e)
				break
			}
		}
		if len(e) > 5 && e[:5] == "HINT_" {
			out = append(out, e)
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

// InstalledVersion returns the directory name (= version) of the plugin
// install at ~/.config/hint/plugins/<short>/<version>/, or empty if absent.
func InstalledVersion(configDir, shortname string) string {
	root := filepath.Join(configDir, "plugins", shortname)
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			return e.Name()
		}
	}
	return ""
}
