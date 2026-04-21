// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

// Package installer installs a project as a Claude Code plugin. Claude Code
// bug #2682 causes MCP servers registered via mcpServers in settings.json or
// ~/.claude.json to fail tool registration; plugin-bundled servers use a
// separate, working code path. This package generates the marketplace +
// plugin manifest files, updates ~/.claude/settings.json with the expected
// extraKnownMarketplaces and enabledPlugins entries, and belt-and-suspenders
// injects the same data into Claude Code's internal cache and registry
// files at ~/.claude/plugins/.
//
// Callers provide an Identity value describing the plugin; the Install
// orchestrator handles all four layers Claude Code looks at.
package installer

// Identity carries the project-specific fields used to produce marketplace
// and plugin manifests. PluginName and PluginDesc are required; other fields
// are filled in by WithDefaults when left blank.
//
// Changing PluginName or MarketplaceName after release breaks existing
// installations — treat them as stable for the lifetime of the plugin.
type Identity struct {
	// PluginName is the plugin's short name (binary name, MCP server key,
	// marketplace plugin entry). Required.
	PluginName string

	// MarketplaceName is the marketplace identifier stored in
	// extraKnownMarketplaces and known_marketplaces.json. Defaults to
	// "<PluginName>-local".
	MarketplaceName string

	// OwnerName appears in marketplace.json owner.name and plugin.json
	// author.name. Defaults to PluginName.
	OwnerName string

	// OwnerEmail appears in marketplace.json owner.email. Defaults to
	// "noreply@<PluginName>.dev".
	OwnerEmail string

	// MarketplaceDesc appears in marketplace.json description. Defaults to
	// "Local <PluginName> plugin marketplace".
	MarketplaceDesc string

	// PluginDesc appears in marketplace.json plugins[].description and
	// plugin.json description. Required — callers must supply a meaningful
	// description of what the plugin does.
	PluginDesc string

	// BinaryName is the executable name used by exec.LookPath when the
	// caller does not supply Config.BinaryPath explicitly. Defaults to
	// PluginName.
	BinaryName string

	// McpArgs are prepended to Config.ExtraArgs when building the args list
	// for .mcp.json. Use this for subcommands that must always run (e.g.
	// ["mcp"] for a binary whose MCP server lives under a subcommand, or
	// ["serve"] for a binary whose top-level serve command is the MCP
	// server). Defaults to nil.
	McpArgs []string

	// LegacyMcpServerName, if non-empty, names an mcpServers entry in
	// ~/.claude/settings.json that Install/Uninstall should remove as part
	// of legacy migration. Leave empty to skip legacy cleanup.
	LegacyMcpServerName string
}

// WithDefaults returns a copy of id with blank fields populated from
// PluginName. PluginName and PluginDesc must already be set — WithDefaults
// does not inject values for them.
func (id Identity) WithDefaults() Identity {
	if id.MarketplaceName == "" {
		id.MarketplaceName = id.PluginName + "-local"
	}
	if id.OwnerName == "" {
		id.OwnerName = id.PluginName
	}
	if id.OwnerEmail == "" {
		id.OwnerEmail = "noreply@" + id.PluginName + ".dev"
	}
	if id.MarketplaceDesc == "" {
		id.MarketplaceDesc = "Local " + id.PluginName + " plugin marketplace"
	}
	if id.BinaryName == "" {
		id.BinaryName = id.PluginName
	}
	return id
}

// PluginKey returns the "<PluginName>@<MarketplaceName>" identifier used as
// the key in enabledPlugins and installed_plugins.json.
func (id Identity) PluginKey() string {
	return id.PluginName + "@" + id.MarketplaceName
}
