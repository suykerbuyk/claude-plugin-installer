// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cacheFilePerm is the file permission used by Claude Code for its internal
// plugin bookkeeping files.
const cacheFilePerm = 0o600

// installedPluginsVersion is the schema version Claude Code currently
// expects for installed_plugins.json (v2 format).
const installedPluginsVersion = 2

// Inject writes the belt-and-suspenders cache directory and registry entries
// that Claude Code would normally populate itself after processing the
// extraKnownMarketplaces entry. Doing it ourselves guards against cases
// where Claude Code's plugin loader fails to materialize the internal state.
//
// Writes:
//   - ~/.claude/plugins/cache/<marketplace>/<plugin>/<version>/.claude-plugin/plugin.json
//   - ~/.claude/plugins/cache/<marketplace>/<plugin>/<version>/.mcp.json
//   - ~/.claude/plugins/known_marketplaces.json  (merged)
//   - ~/.claude/plugins/installed_plugins.json  (merged, v2 schema)
//
// Sibling marketplaces and plugins are preserved. Timestamps are set to
// time.Now().UTC().
func Inject(paths Paths, cfg Config) error {
	binaryPath, err := resolveBinary(cfg.BinaryPath, paths.Identity.BinaryName)
	if err != nil {
		return fmt.Errorf("resolving binary: %w", err)
	}
	version := cfg.Version
	if version == "" {
		version = "0.0.0-dev"
	}

	if err := writeCacheManifests(paths, version, binaryPath); err != nil {
		return fmt.Errorf("writing cache manifests: %w", err)
	}
	now := time.Now().UTC()
	if err := registerMarketplace(paths, now); err != nil {
		return fmt.Errorf("registering marketplace: %w", err)
	}
	if err := registerInstalledPlugin(paths, version, now); err != nil {
		return fmt.Errorf("registering installed plugin: %w", err)
	}
	return nil
}

// Uninject reverses Inject. Missing cache dirs and absent registry entries
// are silently ignored — Uninject is safe to run against partial or empty
// state. Sibling plugins are preserved.
func Uninject(paths Paths) error {
	if err := removeAllCacheVersions(paths); err != nil {
		return fmt.Errorf("removing cache: %w", err)
	}
	if err := unregisterMarketplace(paths); err != nil {
		return fmt.Errorf("unregistering marketplace: %w", err)
	}
	if err := unregisterInstalledPlugin(paths); err != nil {
		return fmt.Errorf("unregistering installed plugin: %w", err)
	}
	return nil
}

// HasCacheInstalled reports whether the cache directory exists for the
// given version and contains both the plugin and mcp manifests.
func HasCacheInstalled(paths Paths, version string) bool {
	for _, p := range []string{paths.CachePluginManifest(version), paths.CacheMcpJson(version)} {
		if _, err := os.Stat(p); err != nil {
			return false
		}
	}
	return true
}

// HasAnyCacheInstalled returns true if at least one version of the plugin
// is present in the cache directory. Used when the current binary's version
// is unknown.
func HasAnyCacheInstalled(paths Paths) bool {
	root := filepath.Join(paths.PluginsDir, "cache", paths.Identity.MarketplaceName, paths.Identity.PluginName)
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			return true
		}
	}
	return false
}

// HasMarketplaceRegistered reports whether known_marketplaces.json contains
// an entry for the plugin's marketplace.
func HasMarketplaceRegistered(paths Paths) (bool, error) {
	m, err := readRegistryFile(paths.KnownMarketplaces)
	if err != nil {
		return false, err
	}
	_, found := m[paths.Identity.MarketplaceName]
	return found, nil
}

// HasInstalledPluginRegistered reports whether installed_plugins.json
// contains an entry for the plugin key.
func HasInstalledPluginRegistered(paths Paths) (bool, error) {
	data, err := os.ReadFile(paths.InstalledPlugins)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var doc installedPluginsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("parsing %s: %w", paths.InstalledPlugins, err)
	}
	_, found := doc.Plugins[paths.Identity.PluginKey()]
	return found, nil
}

func writeCacheManifests(paths Paths, version, binaryPath string) error {
	pluginManifest := buildPluginManifest(paths.Identity, version)
	mcpManifest := buildMcpManifest(paths.Identity, binaryPath, nil)

	if err := writeJSONMode(paths.CachePluginManifest(version), pluginManifest, cacheFilePerm); err != nil {
		return err
	}
	if err := writeJSONMode(paths.CacheMcpJson(version), mcpManifest, cacheFilePerm); err != nil {
		return err
	}
	return nil
}

func removeAllCacheVersions(paths Paths) error {
	root := filepath.Join(paths.PluginsDir, "cache", paths.Identity.MarketplaceName)
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("removing %s: %w", root, err)
	}
	return nil
}

func registerMarketplace(paths Paths, now time.Time) error {
	markets, err := readRegistryFile(paths.KnownMarketplaces)
	if err != nil {
		return err
	}
	markets[paths.Identity.MarketplaceName] = map[string]any{
		"source": map[string]any{
			"source": "directory",
			"path":   paths.MarketplaceRoot,
		},
		"installLocation": paths.MarketplaceRoot,
		"lastUpdated":     now.Format(time.RFC3339Nano),
	}
	return writeJSONMode(paths.KnownMarketplaces, markets, cacheFilePerm)
}

func unregisterMarketplace(paths Paths) error {
	markets, err := readRegistryFile(paths.KnownMarketplaces)
	if err != nil {
		return err
	}
	if _, found := markets[paths.Identity.MarketplaceName]; !found {
		return nil
	}
	delete(markets, paths.Identity.MarketplaceName)
	if len(markets) == 0 {
		if err := os.Remove(paths.KnownMarketplaces); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeJSONMode(paths.KnownMarketplaces, markets, cacheFilePerm)
}

// installedPluginsDoc mirrors the v2 schema of installed_plugins.json.
type installedPluginsDoc struct {
	Version int                                   `json:"version"`
	Plugins map[string][]installedPluginVersionV2 `json:"plugins"`
}

type installedPluginVersionV2 struct {
	Scope       string `json:"scope"`
	InstallPath string `json:"installPath"`
	Version     string `json:"version"`
	InstalledAt string `json:"installedAt"`
	LastUpdated string `json:"lastUpdated"`
}

func registerInstalledPlugin(paths Paths, version string, now time.Time) error {
	doc, err := readInstalledPluginsDoc(paths.InstalledPlugins)
	if err != nil {
		return err
	}

	key := paths.Identity.PluginKey()
	entry := installedPluginVersionV2{
		Scope:       "user",
		InstallPath: paths.CacheVersionDir(version),
		Version:     version,
		InstalledAt: now.Format(time.RFC3339Nano),
		LastUpdated: now.Format(time.RFC3339Nano),
	}

	existing := doc.Plugins[key]
	merged := make([]installedPluginVersionV2, 0, len(existing)+1)
	for _, e := range existing {
		if e.Version == version {
			entry.InstalledAt = e.InstalledAt
			continue
		}
		merged = append(merged, e)
	}
	merged = append(merged, entry)

	if doc.Plugins == nil {
		doc.Plugins = make(map[string][]installedPluginVersionV2)
	}
	doc.Plugins[key] = merged
	doc.Version = installedPluginsVersion

	return writeJSONMode(paths.InstalledPlugins, doc, cacheFilePerm)
}

func unregisterInstalledPlugin(paths Paths) error {
	doc, err := readInstalledPluginsDoc(paths.InstalledPlugins)
	if err != nil {
		return err
	}
	key := paths.Identity.PluginKey()
	if _, found := doc.Plugins[key]; !found {
		return nil
	}
	delete(doc.Plugins, key)
	if len(doc.Plugins) == 0 {
		if err := os.Remove(paths.InstalledPlugins); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeJSONMode(paths.InstalledPlugins, doc, cacheFilePerm)
}

func readRegistryFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if m == nil {
		m = make(map[string]any)
	}
	return m, nil
}

func readInstalledPluginsDoc(path string) (installedPluginsDoc, error) {
	doc := installedPluginsDoc{
		Version: installedPluginsVersion,
		Plugins: make(map[string][]installedPluginVersionV2),
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return doc, nil
		}
		return doc, err
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return installedPluginsDoc{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if doc.Plugins == nil {
		doc.Plugins = make(map[string][]installedPluginVersionV2)
	}
	return doc, nil
}

// writeJSONMode is like writeJSON but with an explicit permission mode.
func writeJSONMode(path string, v any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, mode); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
