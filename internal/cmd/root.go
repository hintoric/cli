// Package cmd contains cobra command implementations for the hint host CLI.
package cmd

import "github.com/spf13/cobra"

// Root returns the top-level cobra command.
// version is injected from main via -ldflags. pubKeyPEM is the embedded
// ECDSA P-256 public key used to verify plugin manifest signatures.
func Root(version string, pubKeyPEM []byte) *cobra.Command {
	root := &cobra.Command{
		Use:   "hint",
		Short: "Hintoric command-line tool",
		Long:  "hint — the Hintoric command-line tool. Plugins extend its surface.",
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = version

	root.AddCommand(versionCommand(version))
	root.AddCommand(updateCommand())
	root.AddCommand(pluginParent(pubKeyPEM))
	root.AddCommand(manifestInternalCommand(pubKeyPEM))

	installDispatch(root, pubKeyPEM)
	return root
}
