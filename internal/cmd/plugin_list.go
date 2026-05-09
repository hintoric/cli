package cmd

import (
	"context"
	"fmt"
	"runtime"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/plugins"
)

// pluginListCommand returns `hint plugin list`.
//
// Prints a tab-aligned table with one row per plugin in the manifest:
//
//	PLUGIN  INSTALLED  LATEST  STATUS
//
// STATUS is "not installed", "update available", or "up to date". No file
// lock is taken since the command is read-only.
func pluginListCommand(pubKeyPEM []byte) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed and available plugins",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			deps, err := newPluginDeps(pubKeyPEM)
			if err != nil {
				return err
			}
			ctx := context.Background()
			pl, err := deps.loader.Get(ctx, false)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "PLUGIN\tINSTALLED\tLATEST\tSTATUS")
			for i := range pl.Plugins {
				p := &pl.Plugins[i]
				installed := plugins.InstalledVersion(deps.configDir, p.Shortname)
				rel := plugins.SelectLatestRelease(p, runtime.GOOS, runtime.GOARCH)
				latest := ""
				if rel != nil {
					latest = rel.Version
				}
				status := "up to date"
				switch {
				case installed == "":
					status = "not installed"
				case latest != "" && installed != latest:
					status = "update available"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Shortname, installed, latest, status)
			}
			return tw.Flush()
		},
	}
}
