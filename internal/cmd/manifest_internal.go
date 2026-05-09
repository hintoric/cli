package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// manifestInternalCommand returns the hidden `hint __manifest` parent command
// used to inspect the cached plugin manifest while debugging. The parent is
// Hidden so it does not appear in `hint --help`, but `hint __manifest --help`
// still works and lists the `refresh` and `show` subcommands.
func manifestInternalCommand(pubKeyPEM []byte) *cobra.Command {
	parent := &cobra.Command{
		Use:    "__manifest",
		Hidden: true,
		Short:  "Inspect the cached plugin manifest (debug)",
	}
	parent.AddCommand(&cobra.Command{
		Use:   "refresh",
		Short: "Bypass cache TTL and re-fetch the manifest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			deps, err := newPluginDeps(pubKeyPEM)
			if err != nil {
				return err
			}
			pl, err := deps.loader.Get(context.Background(), true)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "manifest refreshed: %d plugins\n", len(pl.Plugins))
			return nil
		},
	})
	parent.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print the cached manifest as JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			deps, err := newPluginDeps(pubKeyPEM)
			if err != nil {
				return err
			}
			pl, err := deps.loader.Get(context.Background(), false)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(pl)
		},
	})
	return parent
}
