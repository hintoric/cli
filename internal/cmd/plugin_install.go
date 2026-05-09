package cmd

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/plugins"
)

// pluginInstallCommand returns `hint plugin install <name>[@version]`.
//
// Version resolution priority:
//  1. Explicit @version in the spec.
//  2. cfg.Plugin[name].PinnedVersion.
//  3. Latest release for the current GOOS/GOARCH.
func pluginInstallCommand(pubKeyPEM []byte) *cobra.Command {
	return &cobra.Command{
		Use:   "install <plugin>[@version]",
		Short: "Install a plugin from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, ver := parseSpec(args[0])
			return withLockedDeps(pubKeyPEM, func(ctx context.Context, d *pluginDeps) error {
				pl, err := d.loader.Get(ctx, false)
				if err != nil {
					return err
				}
				p := plugins.FindPlugin(pl, name)
				if p == nil {
					return fmt.Errorf("no plugin named %q in manifest", name)
				}
				if ver == "" {
					if pinned := d.cfg.Plugin[name].PinnedVersion; pinned != "" {
						ver = pinned
					} else {
						rel := plugins.SelectLatestRelease(p, runtime.GOOS, runtime.GOARCH)
						if rel == nil {
							return fmt.Errorf("no release of %q for %s/%s", name, runtime.GOOS, runtime.GOARCH)
						}
						ver = rel.Version
					}
				}
				if err := d.installer.Install(ctx, p, ver); err != nil {
					return err
				}
				addInstalledIfMissing(d.cfg, name)
				if err := d.cfg.Save(d.cfgPath); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "installed %s@%s\n", name, ver)
				return nil
			})
		},
	}
}

// parseSpec splits "name" or "name@version" into (name, version).
func parseSpec(s string) (string, string) {
	if i := strings.IndexByte(s, '@'); i > 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}
