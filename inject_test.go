// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestInject_FreshInstall(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}

	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("Inject error = %v", err)
	}

	if _, err := os.Stat(paths.CachePluginManifest("0.2.0")); err != nil {
		t.Errorf("cache plugin.json missing: %v", err)
	}
	if _, err := os.Stat(paths.CacheMcpJson("0.2.0")); err != nil {
		t.Errorf("cache .mcp.json missing: %v", err)
	}

	info, _ := os.Stat(paths.CachePluginManifest("0.2.0"))
	if info.Mode().Perm() != cacheFilePerm {
		t.Errorf("cache plugin.json perm = %o, want %o", info.Mode().Perm(), cacheFilePerm)
	}

	has, err := HasMarketplaceRegistered(paths)
	if err != nil || !has {
		t.Errorf("HasMarketplaceRegistered = (%v, %v), want (true, nil)", has, err)
	}

	has, err = HasInstalledPluginRegistered(paths)
	if err != nil || !has {
		t.Errorf("HasInstalledPluginRegistered = (%v, %v), want (true, nil)", has, err)
	}
}

func TestInject_CacheMcpJsonIncludesExtraArgs(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{
		Version:    "0.2.0",
		BinaryPath: "/bin/rezbldr",
		ExtraArgs:  []string{"--vault", "/tmp/vault", "--extra-vault", "vibe=/tmp/vibe"},
	}

	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("Inject: %v", err)
	}

	data, err := os.ReadFile(paths.CacheMcpJson("0.2.0"))
	if err != nil {
		t.Fatalf("read cache .mcp.json: %v", err)
	}
	var doc map[string]mcpServerEntry
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse cache .mcp.json: %v", err)
	}
	entry, ok := doc[paths.Identity.PluginName]
	if !ok {
		t.Fatalf("cache .mcp.json missing entry for %q: %v", paths.Identity.PluginName, doc)
	}

	// Cache args must match live-marketplace args exactly: Identity.McpArgs
	// followed by cfg.ExtraArgs. Claude Code launches from the cache, so any
	// divergence makes ExtraArgs invisible to the running process.
	wantArgs := append([]string{}, paths.Identity.McpArgs...)
	wantArgs = append(wantArgs, cfg.ExtraArgs...)
	if len(entry.Args) != len(wantArgs) {
		t.Fatalf("cache args len = %d, want %d (got %v)", len(entry.Args), len(wantArgs), entry.Args)
	}
	for i := range wantArgs {
		if entry.Args[i] != wantArgs[i] {
			t.Errorf("cache args[%d] = %q, want %q", i, entry.Args[i], wantArgs[i])
		}
	}

	// And the cache .mcp.json must exactly equal the live-marketplace
	// .mcp.json that Generate would have written for the same Config.
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	liveData, err := os.ReadFile(paths.McpJson)
	if err != nil {
		t.Fatalf("read live .mcp.json: %v", err)
	}
	var liveDoc map[string]mcpServerEntry
	if err := json.Unmarshal(liveData, &liveDoc); err != nil {
		t.Fatalf("parse live .mcp.json: %v", err)
	}
	if !reflect.DeepEqual(doc, liveDoc) {
		t.Errorf("cache vs live .mcp.json differ\ncache: %v\nlive:  %v", doc, liveDoc)
	}
}

func TestInject_InstalledPluginsShape(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("Inject: %v", err)
	}

	data, err := os.ReadFile(paths.InstalledPlugins)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var doc installedPluginsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Version != installedPluginsVersion {
		t.Errorf("version = %d, want %d", doc.Version, installedPluginsVersion)
	}
	entries := doc.Plugins[id.PluginKey()]
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Scope != "user" {
		t.Errorf("scope = %q, want user", e.Scope)
	}
	if e.Version != "0.2.0" {
		t.Errorf("version = %q, want 0.2.0", e.Version)
	}
	if e.InstallPath != paths.CacheVersionDir("0.2.0") {
		t.Errorf("installPath = %q, want %q", e.InstallPath, paths.CacheVersionDir("0.2.0"))
	}
	if _, err := time.Parse(time.RFC3339Nano, e.InstalledAt); err != nil {
		t.Errorf("installedAt %q not RFC3339Nano: %v", e.InstalledAt, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, e.LastUpdated); err != nil {
		t.Errorf("lastUpdated %q not RFC3339Nano: %v", e.LastUpdated, err)
	}
}

func TestInject_KnownMarketplacesShape(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("Inject: %v", err)
	}

	m, err := readRegistryFile(paths.KnownMarketplaces)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	entry, ok := m[id.MarketplaceName].(map[string]any)
	if !ok {
		t.Fatalf("entry missing for %q: %v", id.MarketplaceName, m)
	}
	if entry["installLocation"] != paths.MarketplaceRoot {
		t.Errorf("installLocation = %v, want %v", entry["installLocation"], paths.MarketplaceRoot)
	}
	source, ok := entry["source"].(map[string]any)
	if !ok {
		t.Fatalf("source missing: %v", entry)
	}
	if source["source"] != "directory" || source["path"] != paths.MarketplaceRoot {
		t.Errorf("source = %v", source)
	}
	if _, err := time.Parse(time.RFC3339Nano, entry["lastUpdated"].(string)); err != nil {
		t.Errorf("lastUpdated %q not RFC3339Nano: %v", entry["lastUpdated"], err)
	}
}

func TestInject_PreservesSiblingMarketplace(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())

	sibling := map[string]any{
		"claude-plugins-official": map[string]any{
			"source":          map[string]any{"source": "github", "repo": "anthropics/claude-plugins-official"},
			"installLocation": "/elsewhere",
			"lastUpdated":     "2026-01-01T00:00:00Z",
		},
	}
	_ = os.MkdirAll(filepath.Dir(paths.KnownMarketplaces), 0o755)
	data, _ := json.MarshalIndent(sibling, "", "  ")
	_ = os.WriteFile(paths.KnownMarketplaces, append(data, '\n'), 0o600)

	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("Inject: %v", err)
	}

	m, _ := readRegistryFile(paths.KnownMarketplaces)
	if _, ok := m["claude-plugins-official"]; !ok {
		t.Error("sibling marketplace was dropped")
	}
	if _, ok := m[paths.Identity.MarketplaceName]; !ok {
		t.Errorf("own marketplace %q not added", paths.Identity.MarketplaceName)
	}
}

func TestInject_PreservesSiblingPlugin(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())

	seed := installedPluginsDoc{
		Version: installedPluginsVersion,
		Plugins: map[string][]installedPluginVersionV2{
			"vibe-vault@vibe-vault-local": {
				{
					Scope:       "user",
					InstallPath: "/elsewhere",
					Version:     "0.9.4",
					InstalledAt: "2026-03-24T00:00:00Z",
					LastUpdated: "2026-03-24T00:00:00Z",
				},
			},
		},
	}
	_ = os.MkdirAll(filepath.Dir(paths.InstalledPlugins), 0o755)
	data, _ := json.MarshalIndent(seed, "", "  ")
	_ = os.WriteFile(paths.InstalledPlugins, append(data, '\n'), 0o600)

	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("Inject: %v", err)
	}

	doc, _ := readInstalledPluginsDoc(paths.InstalledPlugins)
	if _, ok := doc.Plugins["vibe-vault@vibe-vault-local"]; !ok {
		t.Error("sibling plugin was dropped")
	}
	if _, ok := doc.Plugins[paths.Identity.PluginKey()]; !ok {
		t.Errorf("own plugin %q not added", paths.Identity.PluginKey())
	}
}

func TestInject_VersionUpdatePreservesInstalledAt(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}

	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("first: %v", err)
	}
	doc1, _ := readInstalledPluginsDoc(paths.InstalledPlugins)
	firstInstalledAt := doc1.Plugins[paths.Identity.PluginKey()][0].InstalledAt

	time.Sleep(5 * time.Millisecond)

	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("second: %v", err)
	}
	doc2, _ := readInstalledPluginsDoc(paths.InstalledPlugins)
	entries := doc2.Plugins[paths.Identity.PluginKey()]
	if len(entries) != 1 {
		t.Fatalf("entries after re-inject = %d, want 1 (same-version replace)", len(entries))
	}
	if entries[0].InstalledAt != firstInstalledAt {
		t.Errorf("installedAt changed across re-inject: %q → %q",
			firstInstalledAt, entries[0].InstalledAt)
	}
}

func TestInject_MultiVersionSideBySide(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())

	if err := Inject(paths, Config{Version: "0.1.0", BinaryPath: "/bin/rezbldr"}); err != nil {
		t.Fatalf("v1: %v", err)
	}
	if err := Inject(paths, Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}); err != nil {
		t.Fatalf("v2: %v", err)
	}
	doc, _ := readInstalledPluginsDoc(paths.InstalledPlugins)
	entries := doc.Plugins[paths.Identity.PluginKey()]
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
	}
	if !HasCacheInstalled(paths, "0.1.0") || !HasCacheInstalled(paths, "0.2.0") {
		t.Error("both cache versions should be present")
	}
}

func TestUninject_RemovesEverything(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if err := Uninject(paths); err != nil {
		t.Fatalf("Uninject: %v", err)
	}
	if _, err := os.Stat(paths.CacheVersionDir("0.2.0")); !os.IsNotExist(err) {
		t.Error("cache dir should be removed")
	}
	if _, err := os.Stat(paths.KnownMarketplaces); !os.IsNotExist(err) {
		t.Error("known_marketplaces.json should be removed (was only us)")
	}
	if _, err := os.Stat(paths.InstalledPlugins); !os.IsNotExist(err) {
		t.Error("installed_plugins.json should be removed (was only us)")
	}
}

func TestUninject_PreservesSiblings(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())

	sibling := map[string]any{
		"other-marketplace": map[string]any{
			"source":          map[string]any{"source": "directory", "path": "/x"},
			"installLocation": "/x",
			"lastUpdated":     "2026-01-01T00:00:00Z",
		},
	}
	_ = os.MkdirAll(filepath.Dir(paths.KnownMarketplaces), 0o755)
	d, _ := json.MarshalIndent(sibling, "", "  ")
	_ = os.WriteFile(paths.KnownMarketplaces, append(d, '\n'), 0o600)

	siblingDoc := installedPluginsDoc{
		Version: installedPluginsVersion,
		Plugins: map[string][]installedPluginVersionV2{
			"other-plugin@other-marketplace": {{Scope: "user", InstallPath: "/y", Version: "1.0", InstalledAt: "2026-01-01T00:00:00Z", LastUpdated: "2026-01-01T00:00:00Z"}},
		},
	}
	d, _ = json.MarshalIndent(siblingDoc, "", "  ")
	_ = os.WriteFile(paths.InstalledPlugins, append(d, '\n'), 0o600)

	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Inject(paths, cfg); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if err := Uninject(paths); err != nil {
		t.Fatalf("Uninject: %v", err)
	}

	m, _ := readRegistryFile(paths.KnownMarketplaces)
	if _, ok := m["other-marketplace"]; !ok {
		t.Error("sibling marketplace was dropped")
	}
	doc, _ := readInstalledPluginsDoc(paths.InstalledPlugins)
	if _, ok := doc.Plugins["other-plugin@other-marketplace"]; !ok {
		t.Error("sibling plugin was dropped")
	}
	if _, ok := doc.Plugins[paths.Identity.PluginKey()]; ok {
		t.Error("own plugin should have been removed")
	}
}

func TestUninject_NotInstalledIsNoop(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := Uninject(paths); err != nil {
		t.Errorf("Uninject when nothing installed: %v", err)
	}
}

func TestHasCacheInstalled(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if HasCacheInstalled(paths, "0.2.0") {
		t.Error("expected false before install")
	}
	if err := Inject(paths, Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if !HasCacheInstalled(paths, "0.2.0") {
		t.Error("expected true after install")
	}
	if HasCacheInstalled(paths, "9.9.9") {
		t.Error("expected false for wrong version")
	}
}

func TestHasAnyCacheInstalled(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if HasAnyCacheInstalled(paths) {
		t.Error("expected false before install")
	}
	if err := Inject(paths, Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if !HasAnyCacheInstalled(paths) {
		t.Error("expected true after install")
	}
}

func TestHasMarketplaceRegistered_FalseWhenAbsent(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	has, err := HasMarketplaceRegistered(paths)
	if err != nil || has {
		t.Errorf("missing registry: has=%v err=%v", has, err)
	}
}

func TestHasInstalledPluginRegistered_FalseWhenAbsent(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	has, err := HasInstalledPluginRegistered(paths)
	if err != nil || has {
		t.Errorf("missing registry: has=%v err=%v", has, err)
	}
}

func TestReadRegistryFile_InvalidJSON(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	_ = os.MkdirAll(filepath.Dir(paths.KnownMarketplaces), 0o755)
	_ = os.WriteFile(paths.KnownMarketplaces, []byte("{broken"), 0o600)
	if _, err := readRegistryFile(paths.KnownMarketplaces); err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestReadInstalledPluginsDoc_InvalidJSON(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	_ = os.MkdirAll(filepath.Dir(paths.InstalledPlugins), 0o755)
	_ = os.WriteFile(paths.InstalledPlugins, []byte("{broken"), 0o600)
	if _, err := readInstalledPluginsDoc(paths.InstalledPlugins); err == nil {
		t.Error("expected error on invalid JSON")
	}
}
