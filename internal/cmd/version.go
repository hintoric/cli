package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func versionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, runtime, and platform info",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "hint %s\n", version)
			fmt.Fprintf(cmd.OutOrStdout(), "go %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		},
	}
}
