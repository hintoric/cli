package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Uninstall removes a plugin's directory from configDir. Returns an error if
// the plugin isn't installed.
func Uninstall(configDir, shortname string) error {
	root := filepath.Join(configDir, "plugins", shortname)
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("plugin %q is not installed", shortname)
		}
		return err
	}
	return os.RemoveAll(root)
}
