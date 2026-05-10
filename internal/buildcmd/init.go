package buildcmd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

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

// saveManifest writes pl to path atomically: encode into a buffer first, then
// write to a sibling temp file and rename into place. The naive
// os.Create+encode pattern truncates the file on entry — if the encoder
// fails midway (or the process dies), the existing manifest is left as a
// half-written or empty file, which would brick subsequent runs.
func saveManifest(path string, pl *plugins.PluginList) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(pl); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".plugins-*.toml.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
