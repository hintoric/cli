package buildcmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/plugins"
)

func addCommand() *cobra.Command {
	var manifestPath, shortname, version, dir string
	c := &cobra.Command{
		Use:   "add",
		Short: "Ingest plugin binaries and append Release entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if shortname == "" || version == "" || dir == "" {
				return fmt.Errorf("--plugin, --version, and --dir are required")
			}
			pl, err := loadOrInit(manifestPath)
			if err != nil {
				return err
			}
			p := plugins.FindPlugin(pl, shortname)
			if p == nil {
				return fmt.Errorf("plugin %q not found in manifest; run init first", shortname)
			}

			added := 0
			expectedTuples := [][2]string{
				{"darwin", "arm64"}, {"darwin", "amd64"},
				{"linux", "arm64"}, {"linux", "amd64"},
				{"windows", "amd64"},
			}
			for _, tup := range expectedTuples {
				goos, goarch := tup[0], tup[1]
				bin := p.Binary
				if goos == "windows" {
					bin += ".exe"
				}
				path := filepath.Join(dir, goos, goarch, bin)
				data, err := os.ReadFile(path)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: missing %s\n", path)
					continue
				}
				sum := sha256.Sum256(data)
				p.Releases = append(p.Releases, plugins.Release{
					Version: version,
					OS:      goos,
					Arch:    goarch,
					Sum:     hex.EncodeToString(sum[:]),
				})
				added++
			}
			if added == 0 {
				return fmt.Errorf("no binaries found under %s", dir)
			}
			if err := saveManifest(manifestPath, pl); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %d Release(s) for %s@%s\n", added, shortname, version)
			return nil
		},
	}
	c.Flags().StringVar(&manifestPath, "manifest", "plugins.toml", "manifest path")
	c.Flags().StringVar(&shortname, "plugin", "", "plugin shortname (must already exist)")
	c.Flags().StringVar(&version, "version", "", "version string (semver)")
	c.Flags().StringVar(&dir, "dir", "", "root dir; expected layout <dir>/<os>/<arch>/<binary>")
	return c
}
