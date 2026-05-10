package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Uninstall removes a plugin's directory from configDir. Returns an error if
// the plugin isn't installed.
//
// shortname is validated against the plugin-shortname regex before any path
// is constructed — without this guard, "../foo" or "/etc" passed in by a
// caller would reach filepath.Join and let RemoveAll escape configDir.
func Uninstall(configDir, shortname string) error {
	if !ValidShortname(shortname) {
		return fmt.Errorf("invalid plugin name %q", shortname)
	}
	root := filepath.Join(configDir, "plugins", shortname)
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("plugin %q is not installed", shortname)
		}
		return err
	}
	return os.RemoveAll(root)
}
