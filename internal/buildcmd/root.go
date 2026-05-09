// Package buildcmd contains cobra command implementations for hint-build.
package buildcmd

import "github.com/spf13/cobra"

// Root returns the top-level cobra command for the hint-build CLI.
func Root() *cobra.Command {
	c := &cobra.Command{
		Use:   "hint-build",
		Short: "Build, sign, and verify the hint plugin manifest",
	}
	c.AddCommand(initCommand())
	c.AddCommand(addCommand())
	c.AddCommand(signCommand())
	c.AddCommand(verifyCommand())
	return c
}
