package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/plugins"
)

// pluginUninstallCommand returns `hint plugin uninstall <name>`.
// Removes the plugin's install directory and updates the config.
func pluginUninstallCommand(pubKeyPEM []byte) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <plugin>",
		Short: "Uninstall a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withLockedDeps(pubKeyPEM, func(ctx context.Context, d *pluginDeps) error {
				if err := plugins.Uninstall(d.configDir, name); err != nil {
					return err
				}
				removeInstalled(d.cfg, name)
				if err := d.cfg.Save(d.cfgPath); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "uninstalled %s\n", name)
				return nil
			})
		},
	}
}
