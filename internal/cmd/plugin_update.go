package cmd

import (
	"context"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/plugins"
)

// pluginUpdateCommand returns `hint plugin update [<plugin>] [--all] [--force]`.
//
// Without --all, exactly one plugin name must be passed. With --all, no
// positional arg is allowed and every entry in cfg.InstalledPlugins is
// updated. --force bypasses the manifest cache TTL so the next manifest read
// always hits the network.
//
// For each target the command compares the installed version against the
// latest release for the current GOOS/GOARCH and reinstalls if they differ.
// Targets that aren't in the manifest, or have no release for this platform,
// are skipped with a stderr warning instead of aborting the whole run.
func pluginUpdateCommand(pubKeyPEM []byte) *cobra.Command {
	var all bool
	var force bool
	c := &cobra.Command{
		Use:   "update [<plugin>]",
		Short: "Update an installed plugin (or all with --all)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLockedDeps(pubKeyPEM, func(ctx context.Context, d *pluginDeps) error {
				pl, err := d.loader.Get(ctx, force)
				if err != nil {
					return err
				}
				if all && len(args) > 0 {
					return fmt.Errorf("--all and a plugin name are mutually exclusive")
				}
				targets := []string{}
				switch {
				case all:
					targets = append(targets, d.cfg.InstalledPlugins...)
				case len(args) == 1:
					targets = append(targets, args[0])
				default:
					return fmt.Errorf("specify a plugin name or pass --all")
				}
				for _, name := range targets {
					p := plugins.FindPlugin(pl, name)
					if p == nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "skipping %s: not in manifest\n", name)
						continue
					}
					rel := plugins.SelectLatestRelease(p, runtime.GOOS, runtime.GOARCH)
					if rel == nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "skipping %s: no release for this platform\n", name)
						continue
					}
					installed := plugins.InstalledVersion(d.configDir, name)
					if installed == rel.Version {
						fmt.Fprintf(cmd.OutOrStdout(), "%s is up to date (%s)\n", name, installed)
						continue
					}
					if err := d.installer.Install(ctx, p, rel.Version); err != nil {
						return fmt.Errorf("update %s: %w", name, err)
					}
					addInstalledIfMissing(d.cfg, name)
					fmt.Fprintf(cmd.OutOrStdout(), "updated %s: %s -> %s\n", name, installed, rel.Version)
				}
				return d.cfg.Save(d.cfgPath)
			})
		},
	}
	c.Flags().BoolVar(&all, "all", false, "update every installed plugin")
	c.Flags().BoolVar(&force, "force", false, "bypass manifest cache TTL")
	return c
}
