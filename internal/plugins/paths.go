package plugins

import (
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-version"
)

// BinaryExtension returns ".exe" on Windows, empty string elsewhere.
func BinaryExtension() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// InstallPath returns the absolute directory holding the binary for a given
// (plugin, version) pair, rooted at configDir.
func InstallPath(configDir, shortname, ver string) string {
	return filepath.Join(configDir, "plugins", shortname, ver)
}

// BinaryPath returns the absolute path of a plugin's binary on disk.
func BinaryPath(configDir, shortname, ver, binary string) string {
	return filepath.Join(InstallPath(configDir, shortname, ver), binary+BinaryExtension())
}

// SelectLatestRelease picks the highest-version Release matching (os, arch).
// Releases are compared via hashicorp/go-version semantics. Returns nil if none.
func SelectLatestRelease(p *Plugin, os, arch string) *Release {
	var best *Release
	var bestVer *version.Version
	for i := range p.Releases {
		r := &p.Releases[i]
		if r.OS != os || r.Arch != arch {
			continue
		}
		v, err := version.NewVersion(r.Version)
		if err != nil {
			continue
		}
		if bestVer == nil || v.GreaterThan(bestVer) {
			best = r
			bestVer = v
		}
	}
	return best
}

// SelectExactRelease returns the Release matching (version, os, arch) exactly,
// or nil if not found.
func SelectExactRelease(p *Plugin, ver, os, arch string) *Release {
	for i := range p.Releases {
		r := &p.Releases[i]
		if r.Version == ver && r.OS == os && r.Arch == arch {
			return r
		}
	}
	return nil
}

// FindPlugin returns the plugin with the given shortname, or nil.
func FindPlugin(pl *PluginList, shortname string) *Plugin {
	for i := range pl.Plugins {
		if pl.Plugins[i].Shortname == shortname {
			return &pl.Plugins[i]
		}
	}
	return nil
}
