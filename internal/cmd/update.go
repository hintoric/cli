package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func updateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Show how to update hint",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "Run `brew upgrade hint` (or your package manager's equivalent) to update.")
			fmt.Fprintln(cmd.OutOrStdout(), "Self-update is planned for a future release.")
		},
	}
}
