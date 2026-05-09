package buildcmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/plugins"
)

func initCommand() *cobra.Command {
	var manifestPath string
	var binary string
	var shortdesc string
	c := &cobra.Command{
		Use:   "init <shortname>",
		Short: "Add a new Plugin entry to plugins.toml with a fresh magic cookie",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			short := args[0]
			pl, err := loadOrInit(manifestPath)
			if err != nil {
				return err
			}
			if plugins.FindPlugin(pl, short) != nil {
				return fmt.Errorf("plugin %q already exists in manifest", short)
			}
			cookie := make([]byte, 16)
			if _, err := rand.Read(cookie); err != nil {
				return err
			}
			if binary == "" {
				binary = "hint-" + short
			}
			pl.Plugins = append(pl.Plugins, plugins.Plugin{
				Shortname:        short,
				Shortdesc:        shortdesc,
				Binary:           binary,
				MagicCookieValue: hex.EncodeToString(cookie),
			})
			if err := saveManifest(manifestPath, pl); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added plugin %q to %s\n", short, manifestPath)
			return nil
		},
	}
	c.Flags().StringVar(&manifestPath, "manifest", "plugins.toml", "manifest file path")
	c.Flags().StringVar(&binary, "binary", "", "plugin binary filename (default: hint-<shortname>)")
	c.Flags().StringVar(&shortdesc, "desc", "", "short description")
	return c
}

func loadOrInit(path string) (*plugins.PluginList, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if os.IsNotExist(err) {
		return &plugins.PluginList{SchemaVersion: plugins.SchemaVersionCurrent}, nil
	}
	return plugins.ParseManifest(data)
}

func saveManifest(path string, pl *plugins.PluginList) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(pl)
}
