// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstall_FullInstall(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}

	if err := Install(paths, cfg); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !hasMarketplaceFiles(paths) {
		t.Error("marketplace files not fully created")
	}

	has, err := HasSettingsEntries(paths)
	if err != nil || !has {
		t.Errorf("HasSettingsEntries = (%v, %v), want (true, nil)", has, err)
	}

	if !HasCacheInstalled(paths, "0.2.0") {
		t.Error("cache not installed")
	}

	has, err = HasMarketplaceRegistered(paths)
	if err != nil || !has {
		t.Errorf("HasMarketplaceRegistered = (%v, %v)", has, err)
	}
	has, err = HasInstalledPluginRegistered(paths)
	if err != nil || !has {
		t.Errorf("HasInstalledPluginRegistered = (%v, %v)", has, err)
	}
}

func TestInstall_RemovesStaleLegacyEntry(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)

	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{
			id.LegacyMcpServerName: map[string]any{"command": "/stale/path", "args": []any{"serve"}},
		},
	})

	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Install(paths, cfg); err != nil {
		t.Fatalf("Install: %v", err)
	}

	has, err := HasLegacyMcpServer(paths)
	if err != nil {
		t.Fatalf("HasLegacyMcpServer: %v", err)
	}
	if has {
		t.Error("stale legacy entry should have been removed by Install")
	}
}

func TestInstall_Idempotent(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}

	if err := Install(paths, cfg); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := Install(paths, cfg); err != nil {
		t.Fatalf("second: %v", err)
	}

	if !HealthCheck(paths).Healthy() {
		t.Error("health check failed after second install")
	}
}

func TestInstall_SkipSettings(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	// Seed a legacy entry so we can confirm it's left alone when SkipSettings is true.
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{
			id.LegacyMcpServerName: map[string]any{"command": "/stale"},
		},
	})
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr", SkipSettings: true}
	if err := Install(paths, cfg); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Marketplace files present.
	if !hasMarketplaceFiles(paths) {
		t.Error("marketplace files not created")
	}
	// Cache present.
	if !HasCacheInstalled(paths, "0.2.0") {
		t.Error("cache not installed")
	}
	// Settings untouched: our entries are NOT added...
	has, err := HasSettingsEntries(paths)
	if err != nil {
		t.Fatalf("HasSettingsEntries: %v", err)
	}
	if has {
		t.Error("SkipSettings=true: settings entries should not have been added")
	}
	// ...and the legacy entry is NOT removed.
	has, err = HasLegacyMcpServer(paths)
	if err != nil {
		t.Fatalf("HasLegacyMcpServer: %v", err)
	}
	if !has {
		t.Error("SkipSettings=true: legacy entry should have been left alone")
	}
}

func TestUninstall_RemovesEverything(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Install(paths, cfg); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := Uninstall(paths); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if hasMarketplaceFiles(paths) {
		t.Error("marketplace files still present")
	}
	if has, _ := HasSettingsEntries(paths); has {
		t.Error("settings entries still present")
	}
	if HasCacheInstalled(paths, "0.2.0") {
		t.Error("cache still present")
	}
	if has, _ := HasMarketplaceRegistered(paths); has {
		t.Error("marketplace still in known_marketplaces.json")
	}
	if has, _ := HasInstalledPluginRegistered(paths); has {
		t.Error("plugin still in installed_plugins.json")
	}
}

func TestUninstall_AlsoRemovesStaleLegacyEntry(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{
			id.LegacyMcpServerName: map[string]any{"command": "/stale"},
		},
	})
	if err := Uninstall(paths); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if has, _ := HasLegacyMcpServer(paths); has {
		t.Error("legacy entry still present after Uninstall")
	}
}

func TestUninstall_NothingInstalledIsNoop(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := Uninstall(paths); err != nil {
		t.Errorf("Uninstall on empty state: %v", err)
	}
}

func TestUninstallSkipSettings(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	// Install normally so we have settings entries.
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Install(paths, cfg); err != nil {
		t.Fatalf("Install: %v", err)
	}
	// UninstallSkipSettings should remove everything except settings.
	if err := UninstallSkipSettings(paths); err != nil {
		t.Fatalf("UninstallSkipSettings: %v", err)
	}
	if hasMarketplaceFiles(paths) {
		t.Error("marketplace files should be removed")
	}
	if HasCacheInstalled(paths, "0.2.0") {
		t.Error("cache should be removed")
	}
	has, err := HasSettingsEntries(paths)
	if err != nil {
		t.Fatalf("HasSettingsEntries: %v", err)
	}
	if !has {
		t.Error("UninstallSkipSettings should leave settings entries in place")
	}
}

func TestHealthCheck_FreshlyInstalled(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Install(paths, cfg); err != nil {
		t.Fatalf("Install: %v", err)
	}
	s := HealthCheck(paths)
	if !s.Healthy() {
		t.Errorf("not healthy after install: %+v", s)
	}
}

func TestHealthCheck_UninstalledReportsAllFalse(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	s := HealthCheck(paths)
	if s.Healthy() {
		t.Error("empty state reports healthy")
	}
	if s.MarketplaceFiles || s.SettingsEntries || s.CacheInstalled ||
		s.MarketplaceInReg || s.InstalledPluginInReg || s.LegacyMcpServer {
		t.Errorf("expected all flags false, got %+v", s)
	}
}

func TestHealthCheck_DetectsLegacyEntry(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{
			id.LegacyMcpServerName: map[string]any{"command": "/stale"},
		},
	})
	s := HealthCheck(paths)
	if !s.LegacyMcpServer {
		t.Error("HealthCheck did not detect legacy mcpServers entry")
	}
	if s.Healthy() {
		t.Error("Healthy() should be false when legacy entry present")
	}
}

func TestHealthCheck_PropagatesReadError(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	_ = os.MkdirAll(filepath.Dir(paths.Settings), 0o755)
	_ = os.WriteFile(paths.Settings, []byte("{invalid"), 0o644)

	s := HealthCheck(paths)
	if s.FirstError == nil {
		t.Error("expected FirstError to be populated on invalid JSON")
	}
	if s.Healthy() {
		t.Error("Healthy() should be false when FirstError set")
	}
}

func TestHealthCheck_PartialInstall(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := Generate(paths, Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	s := HealthCheck(paths)
	if !s.MarketplaceFiles {
		t.Error("MarketplaceFiles should be true")
	}
	if s.SettingsEntries || s.CacheInstalled || s.MarketplaceInReg || s.InstalledPluginInReg {
		t.Errorf("other flags should be false: %+v", s)
	}
	if s.Healthy() {
		t.Error("partial install should not report healthy")
	}
}

func TestHealthCheck_NonFatalReadErrorsStopAtFirst(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	_ = os.MkdirAll(filepath.Dir(paths.KnownMarketplaces), 0o755)
	_ = os.WriteFile(paths.KnownMarketplaces, []byte("{broken"), 0o600)
	_ = os.WriteFile(paths.InstalledPlugins, []byte("{broken"), 0o600)

	s := HealthCheck(paths)
	if s.FirstError == nil {
		t.Error("expected FirstError to be set when registry files are invalid")
	}
}

func TestHealthCheck_SiblingPluginsIgnored(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.2.0", BinaryPath: "/bin/rezbldr"}
	if err := Install(paths, cfg); err != nil {
		t.Fatalf("Install: %v", err)
	}
	m, _ := readRegistryFile(paths.KnownMarketplaces)
	m["other-local"] = map[string]any{
		"source":          map[string]any{"source": "directory", "path": "/x"},
		"installLocation": "/x",
		"lastUpdated":     "2026-01-01T00:00:00Z",
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	_ = os.WriteFile(paths.KnownMarketplaces, append(data, '\n'), 0o600)

	doc, _ := readInstalledPluginsDoc(paths.InstalledPlugins)
	doc.Plugins["other@other-local"] = []installedPluginVersionV2{{Scope: "user", Version: "1.0", InstallPath: "/y", InstalledAt: "2026-01-01T00:00:00Z", LastUpdated: "2026-01-01T00:00:00Z"}}
	data, _ = json.MarshalIndent(doc, "", "  ")
	_ = os.WriteFile(paths.InstalledPlugins, append(data, '\n'), 0o600)

	s := HealthCheck(paths)
	if !s.Healthy() {
		t.Errorf("sibling plugins should not affect Healthy; got %+v", s)
	}
}
