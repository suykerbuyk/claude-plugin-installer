// Copyright (c) 2026 John Suykerbuyk and SykeTech LTD
// SPDX-License-Identifier: MIT OR Apache-2.0

package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Errorf("file %s does not end with newline", path)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return m
}

func testConfig() Config {
	return Config{
		Version:    "0.2.0",
		BinaryPath: "/tmp/fake-home/.local/bin/rezbldr",
	}
}

func TestGenerate_CreatesAllThreeFiles(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())

	if err := Generate(paths, testConfig()); err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	for _, path := range []string{paths.MarketplaceManifest, paths.PluginManifest, paths.McpJson} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}
}

func TestGenerate_MarketplaceManifestShape(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	if err := Generate(paths, testConfig()); err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	m := readJSONFile(t, paths.MarketplaceManifest)
	if m["$schema"] != marketplaceSchemaURL {
		t.Errorf("$schema = %v, want %v", m["$schema"], marketplaceSchemaURL)
	}
	if m["name"] != id.MarketplaceName {
		t.Errorf("name = %v, want %v", m["name"], id.MarketplaceName)
	}
	if m["description"] != id.MarketplaceDesc {
		t.Errorf("description = %v, want %v", m["description"], id.MarketplaceDesc)
	}

	owner, ok := m["owner"].(map[string]any)
	if !ok {
		t.Fatalf("owner missing or wrong type: %T", m["owner"])
	}
	if owner["name"] != id.OwnerName || owner["email"] != id.OwnerEmail {
		t.Errorf("owner = %v, want name=%q email=%q", owner, id.OwnerName, id.OwnerEmail)
	}

	plugins, ok := m["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		t.Fatalf("plugins = %v, want slice of length 1", m["plugins"])
	}
	p := plugins[0].(map[string]any)
	if p["name"] != id.PluginName || p["source"] != "./"+id.PluginName {
		t.Errorf("plugin ref = %v, want name=%q source=./%s", p, id.PluginName, id.PluginName)
	}
}

func TestGenerate_PluginManifestShape(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	cfg := testConfig()
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	m := readJSONFile(t, paths.PluginManifest)
	if m["name"] != id.PluginName {
		t.Errorf("name = %v, want %v", m["name"], id.PluginName)
	}
	if m["version"] != cfg.Version {
		t.Errorf("version = %v, want %v", m["version"], cfg.Version)
	}
	if m["description"] != id.PluginDesc {
		t.Errorf("description = %v, want %v", m["description"], id.PluginDesc)
	}
}

func TestGenerate_PluginHasAuthor(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	if err := Generate(paths, testConfig()); err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	m := readJSONFile(t, paths.PluginManifest)
	author, ok := m["author"].(map[string]any)
	if !ok {
		t.Fatalf("author missing or wrong type: %T", m["author"])
	}
	if author["name"] != id.OwnerName {
		t.Errorf("author.name = %v, want %v", author["name"], id.OwnerName)
	}
}

func TestGenerate_McpManifestShape(t *testing.T) {
	home := t.TempDir()
	id := testIdentity()
	paths := FromHome(home, id)
	cfg := testConfig()
	cfg.ExtraArgs = []string{"--vault", "/vault/path"}
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	m := readJSONFile(t, paths.McpJson)
	entry, ok := m[id.PluginName].(map[string]any)
	if !ok {
		t.Fatalf("%s entry missing: %v", id.PluginName, m)
	}
	if entry["command"] != cfg.BinaryPath {
		t.Errorf("command = %v, want %v", entry["command"], cfg.BinaryPath)
	}
	args, ok := entry["args"].([]any)
	if !ok {
		t.Fatalf("args missing: %v", entry)
	}
	// testIdentity has McpArgs=["serve"], so the full args is ["serve", "--vault", "/vault/path"].
	wantArgs := []any{"serve", "--vault", "/vault/path"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("args = %v, want %v", args, wantArgs)
	}
}

func TestGenerate_McpManifestNoExtraArgs(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := Generate(paths, testConfig()); err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	m := readJSONFile(t, paths.McpJson)
	entry := m["rezbldr"].(map[string]any)
	args := entry["args"].([]any)
	if !reflect.DeepEqual(args, []any{"serve"}) {
		t.Errorf("args = %v, want [serve]", args)
	}
}

func TestGenerate_McpManifestWithDifferentMcpArgs(t *testing.T) {
	// Simulates vibe-vault-style identity where the MCP server lives under
	// `mcp` subcommand rather than `serve`.
	home := t.TempDir()
	id := Identity{
		PluginName: "vv-like",
		PluginDesc: "test plugin",
		McpArgs:    []string{"mcp"},
	}.WithDefaults()
	paths := FromHome(home, id)
	cfg := Config{Version: "0.1.0", BinaryPath: "/bin/vv-like"}
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	m := readJSONFile(t, paths.McpJson)
	entry := m["vv-like"].(map[string]any)
	args := entry["args"].([]any)
	if !reflect.DeepEqual(args, []any{"mcp"}) {
		t.Errorf("args = %v, want [mcp]", args)
	}
}

func TestGenerate_McpManifestEmptyMcpArgs(t *testing.T) {
	// Identity with no McpArgs — args should contain only ExtraArgs.
	home := t.TempDir()
	id := Identity{
		PluginName: "bare",
		PluginDesc: "test plugin",
	}.WithDefaults()
	paths := FromHome(home, id)
	cfg := Config{Version: "0.1.0", BinaryPath: "/bin/bare"}
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	m := readJSONFile(t, paths.McpJson)
	entry := m["bare"].(map[string]any)
	args := entry["args"].([]any)
	if len(args) != 0 {
		t.Errorf("args = %v, want []", args)
	}
}

func TestGenerate_Idempotent(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := testConfig()

	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("first Generate error = %v", err)
	}
	first, err := os.ReadFile(paths.PluginManifest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("second Generate error = %v", err)
	}
	second, err := os.ReadFile(paths.PluginManifest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("Generate not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestGenerate_VersionUpdate(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := testConfig()

	cfg.Version = "0.1.0"
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	m := readJSONFile(t, paths.PluginManifest)
	if m["version"] != "0.1.0" {
		t.Fatalf("initial version = %v, want 0.1.0", m["version"])
	}

	cfg.Version = "0.2.0"
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate (update) error = %v", err)
	}
	m = readJSONFile(t, paths.PluginManifest)
	if m["version"] != "0.2.0" {
		t.Errorf("updated version = %v, want 0.2.0", m["version"])
	}
}

func TestGenerate_MissingBinaryFallback(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{Version: "0.0.1"}
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	m := readJSONFile(t, paths.McpJson)
	entry := m["rezbldr"].(map[string]any)
	command, ok := entry["command"].(string)
	if !ok || command == "" {
		t.Errorf("command should have been resolved to non-empty string, got %v", entry["command"])
	}
}

func TestGenerate_DefaultVersionWhenMissing(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	cfg := Config{BinaryPath: "/tmp/rezbldr"}
	if err := Generate(paths, cfg); err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	m := readJSONFile(t, paths.PluginManifest)
	if m["version"] != "0.0.0-dev" {
		t.Errorf("default version = %v, want 0.0.0-dev", m["version"])
	}
}

func TestGenerate_CreatesParentDirs(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := Generate(paths, testConfig()); err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	subdirs := []string{
		filepath.Dir(paths.MarketplaceManifest),
		filepath.Dir(paths.PluginManifest),
	}
	for _, d := range subdirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected %s to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}
}

func TestRemoveMarketplace(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := Generate(paths, testConfig()); err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	if _, err := os.Stat(paths.MarketplaceRoot); err != nil {
		t.Fatalf("MarketplaceRoot should exist before removal: %v", err)
	}
	if err := RemoveMarketplace(paths); err != nil {
		t.Fatalf("RemoveMarketplace error = %v", err)
	}
	if _, err := os.Stat(paths.MarketplaceRoot); !os.IsNotExist(err) {
		t.Errorf("expected MarketplaceRoot removed, stat err = %v", err)
	}
}

func TestRemoveMarketplace_NotPresentIsNoop(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	if err := RemoveMarketplace(paths); err != nil {
		t.Errorf("RemoveMarketplace when absent: %v", err)
	}
}

func TestResolveBinary_Explicit(t *testing.T) {
	got, err := resolveBinary("/opt/override/rezbldr", "rezbldr")
	if err != nil {
		t.Fatalf("resolveBinary error = %v", err)
	}
	if got != "/opt/override/rezbldr" {
		t.Errorf("got %q, want explicit path", got)
	}
}

func TestResolveBinary_LookPath(t *testing.T) {
	dir := t.TempDir()
	name := "rezbldr"
	fake := filepath.Join(dir, name)
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("writing fake: %v", err)
	}
	t.Setenv("PATH", dir)

	got, err := resolveBinary("", name)
	if err != nil {
		t.Fatalf("resolveBinary error = %v", err)
	}
	if filepath.Base(got) != name {
		t.Errorf("got %q, want basename %q", got, name)
	}
}

func TestResolveBinary_FallbackToExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := resolveBinary("", "nonexistent-binary-name")
	if err != nil {
		t.Fatalf("resolveBinary error = %v", err)
	}
	if got == "" {
		t.Error("got empty, want os.Executable fallback path")
	}
}

func TestResolveBinary_EmptyBinaryNameFallback(t *testing.T) {
	// Identity without BinaryName should still fall back to os.Executable.
	t.Setenv("PATH", t.TempDir())
	got, err := resolveBinary("", "")
	if err != nil {
		t.Fatalf("resolveBinary error = %v", err)
	}
	if got == "" {
		t.Error("got empty with no BinaryName")
	}
}

func TestWriteJSON_UnwritableParent(t *testing.T) {
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("block"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	target := filepath.Join(blockingFile, "child", "file.json")
	if err := writeJSON(target, map[string]string{"k": "v"}); err == nil {
		t.Error("expected writeJSON to fail when parent path is a regular file")
	}
}

func TestGenerate_PropagatesWriteErrors(t *testing.T) {
	home := t.TempDir()
	paths := FromHome(home, testIdentity())
	blocker := filepath.Dir(paths.MarketplaceRoot)
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(paths.MarketplaceRoot, []byte("blocker"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := Generate(paths, testConfig()); err == nil {
		t.Error("expected Generate to fail when marketplace dir cannot be created")
	}
}
