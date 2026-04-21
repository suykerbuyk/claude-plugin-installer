// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import (
	"os"
	"path/filepath"
)

// Paths bundles every filesystem location touched by the installer for a
// given Identity. Construct with FromHome for test-isolated paths, with
// FromDataHome to honor an explicit XDG_DATA_HOME, or with Default for the
// running user's real home directory.
//
// Identity is captured at construction time; the Paths value is therefore
// plugin-scoped and must not be reused across different plugins.
type Paths struct {
	// Identity the paths were constructed for.
	Identity Identity

	Home string

	// Marketplace source tree under <dataHome>/<PluginName>/claude-plugin/.
	MarketplaceRoot     string
	MarketplaceManifest string
	PluginRoot          string
	PluginManifest      string
	McpJson             string

	// Claude Code state under ~/.claude/.
	ClaudeDir         string
	Settings          string
	PluginsDir        string
	KnownMarketplaces string
	InstalledPlugins  string
}

// FromHome returns a Paths instance rooted at the supplied home directory,
// using the XDG default of <home>/.local/share for the marketplace tree.
// For explicit XDG_DATA_HOME support, use FromDataHome.
func FromHome(home string, id Identity) Paths {
	return FromDataHome(home, filepath.Join(home, ".local", "share"), id)
}

// FromDataHome returns a Paths instance with an explicit XDG data-home
// location, enabling XDG_DATA_HOME support and hermetic tests.
func FromDataHome(home, dataHome string, id Identity) Paths {
	id = id.WithDefaults()
	marketplaceRoot := filepath.Join(dataHome, id.PluginName, "claude-plugin")
	pluginRoot := filepath.Join(marketplaceRoot, id.PluginName)
	claudeDir := filepath.Join(home, ".claude")
	pluginsDir := filepath.Join(claudeDir, "plugins")

	return Paths{
		Identity:            id,
		Home:                home,
		MarketplaceRoot:     marketplaceRoot,
		MarketplaceManifest: filepath.Join(marketplaceRoot, ".claude-plugin", "marketplace.json"),
		PluginRoot:          pluginRoot,
		PluginManifest:      filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"),
		McpJson:             filepath.Join(pluginRoot, ".mcp.json"),
		ClaudeDir:           claudeDir,
		Settings:            filepath.Join(claudeDir, "settings.json"),
		PluginsDir:          pluginsDir,
		KnownMarketplaces:   filepath.Join(pluginsDir, "known_marketplaces.json"),
		InstalledPlugins:    filepath.Join(pluginsDir, "installed_plugins.json"),
	}
}

// Default returns Paths for the current user's home directory, honoring
// XDG_DATA_HOME if set.
func Default(id Identity) (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	return FromDataHome(home, dataHome, id), nil
}

// CacheVersionDir returns the version-scoped cache directory Claude Code
// uses to hold a resolved plugin:
// `<home>/.claude/plugins/cache/<marketplace>/<plugin>/<version>`.
func (p Paths) CacheVersionDir(version string) string {
	return filepath.Join(p.PluginsDir, "cache", p.Identity.MarketplaceName, p.Identity.PluginName, version)
}

// CachePluginManifest returns the cache-side plugin.json path for a version.
func (p Paths) CachePluginManifest(version string) string {
	return filepath.Join(p.CacheVersionDir(version), ".claude-plugin", "plugin.json")
}

// CacheMcpJson returns the cache-side .mcp.json path for a version.
func (p Paths) CacheMcpJson(version string) string {
	return filepath.Join(p.CacheVersionDir(version), ".mcp.json")
}

// pathExists reports whether the file or directory at path is accessible.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
