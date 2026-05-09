// Package plugins implements the hint plugin system: manifest types, fetch +
// signature verification, install/run/uninstall, and the hcplugin gRPC contract.
//
// Production manifests are signed with ECDSA P-256 via AWS KMS asymmetric
// keys; the host embeds the matching public key and verifies on every fetch.
// For local development, a plain PEM private key can sign with the same
// ECDSA-SHA256 wire format.
package plugins

import (
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/BurntSushi/toml"
)

// SchemaVersionCurrent is the manifest schema version this build understands.
// Bumping this is a breaking change.
const SchemaVersionCurrent = 1

// PluginList is the top-level manifest document.
type PluginList struct {
	SchemaVersion int      `toml:"SchemaVersion"`
	GeneratedAt   string   `toml:"GeneratedAt"`
	Plugins       []Plugin `toml:"Plugin"`
}

// Plugin describes a single distributable plugin.
type Plugin struct {
	Shortname        string        `toml:"Shortname"`
	Shortdesc        string        `toml:"Shortdesc"`
	Binary           string        `toml:"Binary"`
	MagicCookieValue string        `toml:"MagicCookieValue"`
	Releases         []Release     `toml:"Release"`
	Commands         []CommandInfo `toml:"Command,omitempty"`
}

// Release is a single (version, os, arch) build of a plugin.
type Release struct {
	Version string `toml:"Version"`
	OS      string `toml:"OS"`
	Arch    string `toml:"Arch"`
	Sum     string `toml:"Sum"` // hex sha256
}

// CommandInfo is metadata for shell completion / help integration.
// Source of truth for the actual CLI surface remains the plugin binary.
type CommandInfo struct {
	Name     string        `toml:"Name"`
	Desc     string        `toml:"Desc,omitempty"`
	Commands []CommandInfo `toml:"Command,omitempty"`
}

// ParseManifest parses a TOML-encoded manifest into a PluginList.
// It does not enforce schema constraints — see ValidateManifest for that.
func ParseManifest(data []byte) (*PluginList, error) {
	var pl PluginList
	if _, err := toml.Decode(string(data), &pl); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &pl, nil
}

var shortnameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ValidateManifest enforces schema constraints on a parsed manifest.
// It does not verify signatures — see VerifyManifestSignature for that.
func ValidateManifest(pl *PluginList) error {
	if pl == nil {
		return fmt.Errorf("manifest is nil")
	}
	if pl.SchemaVersion != SchemaVersionCurrent {
		return fmt.Errorf("unsupported SchemaVersion %d (this build understands %d)",
			pl.SchemaVersion, SchemaVersionCurrent)
	}
	if len(pl.Plugins) == 0 {
		return fmt.Errorf("manifest has no plugins")
	}
	seen := map[string]bool{}
	for i, p := range pl.Plugins {
		if !shortnameRE.MatchString(p.Shortname) {
			return fmt.Errorf("plugin[%d]: invalid Shortname %q (must match %s)",
				i, p.Shortname, shortnameRE.String())
		}
		if seen[p.Shortname] {
			return fmt.Errorf("plugin[%d]: duplicate Shortname %q", i, p.Shortname)
		}
		seen[p.Shortname] = true

		if p.Binary == "" {
			return fmt.Errorf("plugin %q: empty Binary", p.Shortname)
		}
		if p.MagicCookieValue == "" {
			return fmt.Errorf("plugin %q: empty MagicCookieValue", p.Shortname)
		}
		if len(p.Releases) == 0 {
			return fmt.Errorf("plugin %q: no Releases", p.Shortname)
		}
		for j, r := range p.Releases {
			if r.Version == "" || r.OS == "" || r.Arch == "" {
				return fmt.Errorf("plugin %q release[%d]: missing version/os/arch", p.Shortname, j)
			}
			if _, err := hex.DecodeString(r.Sum); err != nil {
				return fmt.Errorf("plugin %q release[%d]: Sum is not hex: %w", p.Shortname, j, err)
			}
		}
	}
	return nil
}
