package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/plugins"
)

// pluginDispatch is wired as the cobra command dispatcher for unknown commands.
// It returns true if it handled the invocation (in which case it also calls
// os.Exit), false if cobra should fall through to its default unknown-command
// error.
func pluginDispatch(pubKeyPEM []byte, args []string) bool {
	if len(args) == 0 {
		return false
	}
	name := args[0]
	if name == "help" || name == "version" || name == "update" || name == "plugin" || name == "__manifest" {
		return false
	}
	deps, err := newPluginDeps(pubKeyPEM)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	pl, err := deps.loader.Get(context.Background(), false)
	if err != nil {
		// not a plugin we know about → let cobra emit "unknown command"
		return false
	}
	p := plugins.FindPlugin(pl, name)
	if p == nil {
		return false
	}
	code, err := deps.runner.Run(context.Background(), p, args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
	return true
}

// installDispatch wires pluginDispatch into cobra's root command. It sets
// root.RunE so that when cobra fails to match a built-in subcommand, our
// dispatcher gets a chance to route the invocation to a plugin.
func installDispatch(root *cobra.Command, pubKeyPEM []byte) {
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if pluginDispatch(pubKeyPEM, args) {
			return nil
		}
		// Fall through to help.
		return cmd.Help()
	}
	root.Args = cobra.ArbitraryArgs
}
