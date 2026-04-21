// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import "testing"

func TestIdentity_WithDefaults_FillsBlanks(t *testing.T) {
	id := Identity{
		PluginName: "rezbldr",
		PluginDesc: "MCP server for deterministic resume pipeline operations",
	}.WithDefaults()

	if id.MarketplaceName != "rezbldr-local" {
		t.Errorf("MarketplaceName = %q, want %q", id.MarketplaceName, "rezbldr-local")
	}
	if id.OwnerName != "rezbldr" {
		t.Errorf("OwnerName = %q, want %q", id.OwnerName, "rezbldr")
	}
	if id.OwnerEmail != "noreply@rezbldr.dev" {
		t.Errorf("OwnerEmail = %q, want %q", id.OwnerEmail, "noreply@rezbldr.dev")
	}
	if id.MarketplaceDesc != "Local rezbldr plugin marketplace" {
		t.Errorf("MarketplaceDesc = %q", id.MarketplaceDesc)
	}
	if id.BinaryName != "rezbldr" {
		t.Errorf("BinaryName = %q, want %q", id.BinaryName, "rezbldr")
	}
}

func TestIdentity_WithDefaults_PreservesExplicit(t *testing.T) {
	id := Identity{
		PluginName:      "vibe-vault",
		MarketplaceName: "custom-market",
		OwnerName:       "Custom Owner",
		OwnerEmail:      "a@b.c",
		MarketplaceDesc: "custom",
		PluginDesc:      "desc",
		BinaryName:      "vv",
		McpArgs:         []string{"mcp"},
	}.WithDefaults()

	if id.MarketplaceName != "custom-market" {
		t.Errorf("MarketplaceName overwritten: %q", id.MarketplaceName)
	}
	if id.OwnerName != "Custom Owner" {
		t.Errorf("OwnerName overwritten: %q", id.OwnerName)
	}
	if id.OwnerEmail != "a@b.c" {
		t.Errorf("OwnerEmail overwritten: %q", id.OwnerEmail)
	}
	if id.MarketplaceDesc != "custom" {
		t.Errorf("MarketplaceDesc overwritten: %q", id.MarketplaceDesc)
	}
	if id.BinaryName != "vv" {
		t.Errorf("BinaryName overwritten: %q", id.BinaryName)
	}
	if len(id.McpArgs) != 1 || id.McpArgs[0] != "mcp" {
		t.Errorf("McpArgs = %v", id.McpArgs)
	}
}

func TestIdentity_PluginKey(t *testing.T) {
	id := Identity{PluginName: "rezbldr"}.WithDefaults()
	if got := id.PluginKey(); got != "rezbldr@rezbldr-local" {
		t.Errorf("PluginKey = %q, want %q", got, "rezbldr@rezbldr-local")
	}

	id2 := Identity{PluginName: "vibe-vault", MarketplaceName: "vibe-vault-local"}
	if got := id2.PluginKey(); got != "vibe-vault@vibe-vault-local" {
		t.Errorf("PluginKey = %q, want %q", got, "vibe-vault@vibe-vault-local")
	}
}

// testIdentity returns an Identity that mirrors rezbldr's historical constants —
// the reference identity most other tests use.
func testIdentity() Identity {
	return Identity{
		PluginName:          "rezbldr",
		PluginDesc:          "MCP server for deterministic resume pipeline operations",
		McpArgs:             []string{"serve"},
		LegacyMcpServerName: "rezbldr",
	}.WithDefaults()
}
