// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Config captures the inputs needed to generate the plugin marketplace.
// Identity lives on Paths; Config carries per-invocation state.
type Config struct {
	// Version is the plugin version (typically the binary's version).
	Version string
	// BinaryPath is the absolute path to the binary Claude Code will launch.
	// If empty, Generate resolves it via exec.LookPath(identity.BinaryName),
	// falling back to os.Executable().
	BinaryPath string
	// ExtraArgs are appended after Identity.McpArgs in .mcp.json (e.g.
	// ["--vault", path] — used to pass runtime flags to the MCP server).
	ExtraArgs []string
	// SkipSettings, when true, causes Install and Uninstall to leave
	// ~/.claude/settings.json untouched. Use this when the caller manages
	// settings.json directly (e.g., to coordinate with other Claude Code
	// state that shares the file).
	SkipSettings bool
}

const marketplaceSchemaURL = "https://anthropic.com/claude-code/marketplace.schema.json"

// Generate writes marketplace.json, plugin.json, and .mcp.json under the
// marketplace tree described by paths, using cfg for per-invocation fields
// and paths.Identity for project-specific fields. Parent directories are
// created with 0o755; files written with 0o644. The function is idempotent:
// repeated calls with the same inputs produce byte-identical files.
func Generate(paths Paths, cfg Config) error {
	binaryPath, err := resolveBinary(cfg.BinaryPath, paths.Identity.BinaryName)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	version := cfg.Version
	if version == "" {
		version = "0.0.0-dev"
	}

	marketplace := buildMarketplaceManifest(paths.Identity)
	plugin := buildPluginManifest(paths.Identity, version)
	mcp := buildMcpManifest(paths.Identity, binaryPath, cfg.ExtraArgs)

	if err := writeJSON(paths.MarketplaceManifest, marketplace); err != nil {
		return fmt.Errorf("writing marketplace.json: %w", err)
	}
	if err := writeJSON(paths.PluginManifest, plugin); err != nil {
		return fmt.Errorf("writing plugin.json: %w", err)
	}
	if err := writeJSON(paths.McpJson, mcp); err != nil {
		return fmt.Errorf("writing .mcp.json: %w", err)
	}
	return nil
}

// RemoveMarketplace deletes the entire marketplace tree at paths.MarketplaceRoot.
// Returns nil if the tree does not exist.
func RemoveMarketplace(paths Paths) error {
	if err := os.RemoveAll(paths.MarketplaceRoot); err != nil {
		return fmt.Errorf("removing %s: %w", paths.MarketplaceRoot, err)
	}
	return nil
}

func resolveBinary(explicit, binaryName string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if binaryName != "" {
		if found, err := exec.LookPath(binaryName); err == nil {
			abs, aerr := filepath.Abs(found)
			if aerr == nil {
				return abs, nil
			}
			return found, nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("os.Executable: %w", err)
	}
	return exe, nil
}

// marketplaceManifest is the JSON shape Claude Code expects at
// .claude-plugin/marketplace.json. Field ordering here is preserved in
// encoding/json output via struct tag order.
type marketplaceManifest struct {
	Schema      string                 `json:"$schema"`
	Description string                 `json:"description"`
	Name        string                 `json:"name"`
	Owner       marketplaceOwner       `json:"owner"`
	Plugins     []marketplacePluginRef `json:"plugins"`
}

type marketplaceOwner struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type marketplacePluginRef struct {
	Description string `json:"description"`
	Name        string `json:"name"`
	Source      string `json:"source"`
}

// pluginManifest is the JSON shape at <plugin>/.claude-plugin/plugin.json.
type pluginManifest struct {
	Author      pluginAuthor `json:"author"`
	Description string       `json:"description"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
}

type pluginAuthor struct {
	Name string `json:"name"`
}

// mcpServerEntry matches the shape used inside .mcp.json (one entry per server).
type mcpServerEntry struct {
	Args    []string `json:"args"`
	Command string   `json:"command"`
}

func buildMarketplaceManifest(id Identity) marketplaceManifest {
	return marketplaceManifest{
		Schema:      marketplaceSchemaURL,
		Description: id.MarketplaceDesc,
		Name:        id.MarketplaceName,
		Owner: marketplaceOwner{
			Email: id.OwnerEmail,
			Name:  id.OwnerName,
		},
		Plugins: []marketplacePluginRef{
			{
				Description: id.PluginDesc,
				Name:        id.PluginName,
				Source:      "./" + id.PluginName,
			},
		},
	}
}

func buildPluginManifest(id Identity, version string) pluginManifest {
	return pluginManifest{
		Author:      pluginAuthor{Name: id.OwnerName},
		Description: id.PluginDesc,
		Name:        id.PluginName,
		Version:     version,
	}
}

func buildMcpManifest(id Identity, binaryPath string, extraArgs []string) map[string]mcpServerEntry {
	args := make([]string, 0, len(id.McpArgs)+len(extraArgs))
	args = append(args, id.McpArgs...)
	args = append(args, extraArgs...)
	return map[string]mcpServerEntry{
		id.PluginName: {
			Args:    args,
			Command: binaryPath,
		},
	}
}

// writeJSON marshals v as pretty-printed JSON (2-space indent) with a trailing
// newline, creating parent directories as needed.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
