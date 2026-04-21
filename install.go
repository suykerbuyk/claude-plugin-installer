// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import "fmt"

// Install runs the full plugin provisioning sequence:
//   - Generates the marketplace directory under <dataHome>/<plugin>/claude-plugin/.
//   - Adds extraKnownMarketplaces + enabledPlugins entries to
//     ~/.claude/settings.json (unless Config.SkipSettings is true).
//   - Injects cache manifests + registry entries into ~/.claude/plugins/.
//   - Removes any stale mcpServers entry named by
//     Identity.LegacyMcpServerName (unless Config.SkipSettings is true or
//     the legacy name is empty).
//
// Callers should run Install after the binary has been copied to its final
// location (so exec.LookPath / os.Executable resolve correctly). Errors
// returned are wrapped with step context.
func Install(paths Paths, cfg Config) error {
	if err := Generate(paths, cfg); err != nil {
		return fmt.Errorf("generating marketplace: %w", err)
	}
	if !cfg.SkipSettings {
		if err := AddSettingsEntries(paths); err != nil {
			return fmt.Errorf("writing settings entries: %w", err)
		}
	}
	if err := Inject(paths, cfg); err != nil {
		return fmt.Errorf("injecting cache: %w", err)
	}
	if !cfg.SkipSettings {
		if _, err := RemoveLegacyMcpServer(paths); err != nil {
			return fmt.Errorf("removing legacy mcpServers entry: %w", err)
		}
	}
	return nil
}

// Uninstall reverses Install. All steps are idempotent — Uninstall is safe
// to run against partial or empty state. The order is deliberately the
// reverse of Install: registries and cache first (so Claude Code stops
// seeing the plugin), then settings, then the marketplace directory.
//
// Unlike Install, Uninstall does not accept a Config; it uses the Identity
// bound to paths to decide what to remove. Callers that want to skip
// settings removal should handle that themselves.
func Uninstall(paths Paths) error {
	return uninstall(paths, false)
}

// UninstallSkipSettings is like Uninstall but leaves ~/.claude/settings.json
// untouched. Use this when the caller manages settings.json directly.
func UninstallSkipSettings(paths Paths) error {
	return uninstall(paths, true)
}

func uninstall(paths Paths, skipSettings bool) error {
	if err := Uninject(paths); err != nil {
		return fmt.Errorf("uninjecting cache: %w", err)
	}
	if !skipSettings {
		if err := RemoveSettingsEntries(paths); err != nil {
			return fmt.Errorf("removing settings entries: %w", err)
		}
	}
	if err := RemoveMarketplace(paths); err != nil {
		return fmt.Errorf("removing marketplace: %w", err)
	}
	if !skipSettings {
		if _, err := RemoveLegacyMcpServer(paths); err != nil {
			return fmt.Errorf("removing legacy mcpServers entry: %w", err)
		}
	}
	return nil
}

// Status describes the result of a HealthCheck call. Use for building CLI
// status reports.
type Status struct {
	MarketplaceFiles     bool  // all three marketplace/plugin/mcp JSON files present
	SettingsEntries      bool  // extraKnownMarketplaces + enabledPlugins configured
	CacheInstalled       bool  // at least one cache version present
	MarketplaceInReg     bool  // known_marketplaces.json entry present
	InstalledPluginInReg bool  // installed_plugins.json entry present
	LegacyMcpServer      bool  // stale mcpServers entry still present (warn)
	FirstError           error // first error encountered reading state
}

// Healthy reports whether all expected state is present and no legacy
// artifacts remain.
func (s Status) Healthy() bool {
	return s.MarketplaceFiles &&
		s.SettingsEntries &&
		s.CacheInstalled &&
		s.MarketplaceInReg &&
		s.InstalledPluginInReg &&
		!s.LegacyMcpServer &&
		s.FirstError == nil
}

// HealthCheck returns a Status describing the current installation state.
// Non-fatal read errors are captured in FirstError but do not prevent other
// fields from being populated.
func HealthCheck(paths Paths) Status {
	var s Status

	s.MarketplaceFiles = hasMarketplaceFiles(paths)

	if ok, err := HasSettingsEntries(paths); err != nil {
		s.FirstError = err
	} else {
		s.SettingsEntries = ok
	}

	s.CacheInstalled = HasAnyCacheInstalled(paths)

	if ok, err := HasMarketplaceRegistered(paths); err != nil && s.FirstError == nil {
		s.FirstError = err
	} else {
		s.MarketplaceInReg = ok
	}

	if ok, err := HasInstalledPluginRegistered(paths); err != nil && s.FirstError == nil {
		s.FirstError = err
	} else {
		s.InstalledPluginInReg = ok
	}

	if ok, err := HasLegacyMcpServer(paths); err != nil && s.FirstError == nil {
		s.FirstError = err
	} else {
		s.LegacyMcpServer = ok
	}

	return s
}

func hasMarketplaceFiles(paths Paths) bool {
	for _, p := range []string{paths.MarketplaceManifest, paths.PluginManifest, paths.McpJson} {
		if !pathExists(p) {
			return false
		}
	}
	return true
}
