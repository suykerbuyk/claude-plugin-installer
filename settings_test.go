// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSettingsForTest(t *testing.T, path string, v map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestAddSettingsEntries_FreshFile(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())

	if err := AddSettingsEntries(paths); err != nil {
		t.Fatalf("AddSettingsEntries error = %v", err)
	}

	got := readJSONFile(t, paths.Settings)
	markets := got[settingsKeyExtraKnownMarketplaces].(map[string]any)
	entry := markets[paths.Identity.MarketplaceName].(map[string]any)
	source := entry["source"].(map[string]any)
	if source["path"] != paths.MarketplaceRoot {
		t.Errorf("source.path = %v, want %v", source["path"], paths.MarketplaceRoot)
	}
	if source["source"] != "directory" {
		t.Errorf("source.source = %v, want directory", source["source"])
	}

	plugins := got[settingsKeyEnabledPlugins].(map[string]any)
	if plugins[paths.Identity.PluginKey()] != true {
		t.Errorf("enabledPlugins[%q] = %v, want true", paths.Identity.PluginKey(), plugins[paths.Identity.PluginKey()])
	}
}

func TestAddSettingsEntries_PreservesSiblings(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyExtraKnownMarketplaces: map[string]any{
			"vibe-vault-local": map[string]any{
				"source": map[string]any{"path": "/elsewhere", "source": "directory"},
			},
		},
		settingsKeyEnabledPlugins: map[string]any{
			"gopls-lsp@claude-plugins-official": true,
			"vibe-vault@vibe-vault-local":       true,
		},
		"hooks": map[string]any{
			"Stop": []any{map[string]any{"matcher": ""}},
		},
	})

	if err := AddSettingsEntries(paths); err != nil {
		t.Fatalf("AddSettingsEntries error = %v", err)
	}

	got := readJSONFile(t, paths.Settings)

	markets := got[settingsKeyExtraKnownMarketplaces].(map[string]any)
	if _, found := markets["vibe-vault-local"]; !found {
		t.Error("sibling marketplace vibe-vault-local was dropped")
	}
	if _, found := markets[paths.Identity.MarketplaceName]; !found {
		t.Errorf("own marketplace %q not added", paths.Identity.MarketplaceName)
	}

	plugins := got[settingsKeyEnabledPlugins].(map[string]any)
	for _, sib := range []string{"gopls-lsp@claude-plugins-official", "vibe-vault@vibe-vault-local"} {
		if plugins[sib] != true {
			t.Errorf("sibling plugin %q not preserved, got %v", sib, plugins[sib])
		}
	}
	if plugins[paths.Identity.PluginKey()] != true {
		t.Errorf("own plugin %q not added", paths.Identity.PluginKey())
	}

	if _, ok := got["hooks"].(map[string]any); !ok {
		t.Error("unrelated 'hooks' key was dropped")
	}
}

func TestAddSettingsEntries_Idempotent(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := AddSettingsEntries(paths); err != nil {
		t.Fatalf("first: %v", err)
	}
	first, _ := os.ReadFile(paths.Settings)
	if err := AddSettingsEntries(paths); err != nil {
		t.Fatalf("second: %v", err)
	}
	second, _ := os.ReadFile(paths.Settings)
	if string(first) != string(second) {
		t.Errorf("AddSettingsEntries not idempotent\nfirst: %s\nsecond: %s", first, second)
	}
}

func TestRemoveSettingsEntries_RemovesOwnEntriesPreservesOthers(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := AddSettingsEntries(paths); err != nil {
		t.Fatalf("setup AddSettingsEntries: %v", err)
	}
	settings, _ := readSettings(paths.Settings)
	markets := settings[settingsKeyExtraKnownMarketplaces].(map[string]any)
	markets["other-local"] = map[string]any{"source": map[string]any{"path": "/x", "source": "directory"}}
	plugins := settings[settingsKeyEnabledPlugins].(map[string]any)
	plugins["other-plugin@other-local"] = true
	if err := writeSettings(paths.Settings, settings); err != nil {
		t.Fatalf("re-write settings: %v", err)
	}

	if err := RemoveSettingsEntries(paths); err != nil {
		t.Fatalf("RemoveSettingsEntries error = %v", err)
	}

	got := readJSONFile(t, paths.Settings)
	gotMarkets := got[settingsKeyExtraKnownMarketplaces].(map[string]any)
	if _, found := gotMarkets[paths.Identity.MarketplaceName]; found {
		t.Errorf("own marketplace %q not removed", paths.Identity.MarketplaceName)
	}
	if _, found := gotMarkets["other-local"]; !found {
		t.Error("sibling marketplace was removed")
	}
	gotPlugins := got[settingsKeyEnabledPlugins].(map[string]any)
	if _, found := gotPlugins[paths.Identity.PluginKey()]; found {
		t.Errorf("own plugin %q not removed", paths.Identity.PluginKey())
	}
	if _, found := gotPlugins["other-plugin@other-local"]; !found {
		t.Error("sibling plugin was removed")
	}
}

func TestRemoveSettingsEntries_FileMissingIsNoop(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := RemoveSettingsEntries(paths); err != nil {
		t.Errorf("RemoveSettingsEntries on missing file: %v", err)
	}
	if _, err := os.Stat(paths.Settings); !os.IsNotExist(err) {
		t.Error("RemoveSettingsEntries should not create the file")
	}
}

func TestRemoveSettingsEntries_NoEntriesIsNoop(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	writeSettingsForTest(t, paths.Settings, map[string]any{"unrelated": "value"})
	beforeMtime := fileModTime(t, paths.Settings)
	if err := RemoveSettingsEntries(paths); err != nil {
		t.Errorf("RemoveSettingsEntries: %v", err)
	}
	afterMtime := fileModTime(t, paths.Settings)
	if !beforeMtime.Equal(afterMtime) {
		t.Error("settings file was rewritten despite no entries to remove")
	}
}

func TestRemoveSettingsEntries_EmptiesMapsDelete(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := AddSettingsEntries(paths); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RemoveSettingsEntries(paths); err != nil {
		t.Fatalf("RemoveSettingsEntries: %v", err)
	}
	got := readJSONFile(t, paths.Settings)
	if _, found := got[settingsKeyExtraKnownMarketplaces]; found {
		t.Error("empty extraKnownMarketplaces should be deleted")
	}
	if _, found := got[settingsKeyEnabledPlugins]; found {
		t.Error("empty enabledPlugins should be deleted")
	}
}

func TestRemoveLegacyMcpServer_RemovesAndPreservesSiblings(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{
			id.LegacyMcpServerName: map[string]any{"command": "/old/path", "args": []any{"serve"}},
			"other":                map[string]any{"command": "/other"},
		},
	})
	removed, err := RemoveLegacyMcpServer(paths)
	if err != nil {
		t.Fatalf("RemoveLegacyMcpServer: %v", err)
	}
	if !removed {
		t.Error("expected removed=true")
	}
	got := readJSONFile(t, paths.Settings)
	servers := got[settingsKeyMcpServers].(map[string]any)
	if _, found := servers[id.LegacyMcpServerName]; found {
		t.Error("legacy entry was not removed")
	}
	if _, found := servers["other"]; !found {
		t.Error("sibling server 'other' was dropped")
	}
}

func TestRemoveLegacyMcpServer_DeletesEmptyMap(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{
			id.LegacyMcpServerName: map[string]any{"command": "/old/path"},
		},
	})
	if _, err := RemoveLegacyMcpServer(paths); err != nil {
		t.Fatalf("RemoveLegacyMcpServer: %v", err)
	}
	got := readJSONFile(t, paths.Settings)
	if _, found := got[settingsKeyMcpServers]; found {
		t.Error("empty mcpServers map should be deleted entirely")
	}
}

func TestRemoveLegacyMcpServer_MissingFileNoop(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	removed, err := RemoveLegacyMcpServer(paths)
	if err != nil {
		t.Fatalf("RemoveLegacyMcpServer: %v", err)
	}
	if removed {
		t.Error("expected removed=false for missing file")
	}
}

func TestRemoveLegacyMcpServer_AbsentEntryNoop(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{"other": map[string]any{}},
	})
	removed, err := RemoveLegacyMcpServer(paths)
	if err != nil {
		t.Fatalf("RemoveLegacyMcpServer: %v", err)
	}
	if removed {
		t.Error("expected removed=false when legacy not present")
	}
}

func TestRemoveLegacyMcpServer_EmptyLegacyNameNoop(t *testing.T) {
	// Identity with no LegacyMcpServerName set — even if there's an entry
	// matching the plugin name, it must NOT be touched.
	home := t.TempDir()
	id := Identity{PluginName: "rezbldr", PluginDesc: "x"}.WithDefaults()
	if id.LegacyMcpServerName != "" {
		t.Fatalf("sanity: default LegacyMcpServerName should be empty, got %q", id.LegacyMcpServerName)
	}
	paths := FromHome(home, id)
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{
			"rezbldr": map[string]any{"command": "/bin/rezbldr"},
		},
	})
	removed, err := RemoveLegacyMcpServer(paths)
	if err != nil {
		t.Fatalf("RemoveLegacyMcpServer: %v", err)
	}
	if removed {
		t.Error("expected removed=false when LegacyMcpServerName empty")
	}
	got := readJSONFile(t, paths.Settings)
	if _, found := got[settingsKeyMcpServers].(map[string]any)["rezbldr"]; !found {
		t.Error("entry was removed despite empty LegacyMcpServerName")
	}
}

func TestHasLegacyMcpServer(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)

	has, err := HasLegacyMcpServer(paths)
	if err != nil || has {
		t.Errorf("missing file: has=%v err=%v", has, err)
	}

	writeSettingsForTest(t, paths.Settings, map[string]any{"other": true})
	has, err = HasLegacyMcpServer(paths)
	if err != nil || has {
		t.Errorf("no entry: has=%v err=%v", has, err)
	}

	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{id.LegacyMcpServerName: map[string]any{}},
	})
	has, err = HasLegacyMcpServer(paths)
	if err != nil || !has {
		t.Errorf("with entry: has=%v err=%v", has, err)
	}
}

func TestHasLegacyMcpServer_EmptyLegacyNameIsFalse(t *testing.T) {
	home := t.TempDir()
	id := Identity{PluginName: "rezbldr", PluginDesc: "x"}.WithDefaults()
	paths := FromHome(home, id)
	writeSettingsForTest(t, paths.Settings, map[string]any{
		settingsKeyMcpServers: map[string]any{"rezbldr": map[string]any{}},
	})
	has, err := HasLegacyMcpServer(paths)
	if err != nil || has {
		t.Errorf("with empty legacy name: has=%v err=%v", has, err)
	}
}

func TestHasSettingsEntries(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())

	has, err := HasSettingsEntries(paths)
	if err != nil || has {
		t.Errorf("missing: has=%v err=%v", has, err)
	}

	if err := AddSettingsEntries(paths); err != nil {
		t.Fatalf("add: %v", err)
	}
	has, err = HasSettingsEntries(paths)
	if err != nil || !has {
		t.Errorf("after add: has=%v err=%v", has, err)
	}

	settings, _ := readSettings(paths.Settings)
	markets := settings[settingsKeyExtraKnownMarketplaces].(map[string]any)
	entry := markets[paths.Identity.MarketplaceName].(map[string]any)
	entry["source"].(map[string]any)["path"] = "/wrong/path"
	_ = writeSettings(paths.Settings, settings)
	has, err = HasSettingsEntries(paths)
	if err != nil || has {
		t.Errorf("corrupted path: has=%v err=%v", has, err)
	}
}

func TestReadSettings_InvalidJSON(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := os.MkdirAll(filepath.Dir(paths.Settings), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(paths.Settings, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readSettings(paths.Settings); err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func fileModTime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.ModTime()
}
