# claude-plugin-installer

A Go library for installing a project as a Claude Code plugin.

This handles the four layers Claude Code requires for a local plugin to load:

1. **Marketplace tree** under `~/.local/share/<plugin>/claude-plugin/` with
   `marketplace.json`, `plugin.json`, and `.mcp.json` manifests.
2. **Settings bridge** in `~/.claude/settings.json` — the
   `extraKnownMarketplaces` and `enabledPlugins` entries Claude Code reads
   on startup to discover local marketplaces.
3. **Cache injection** under `~/.claude/plugins/cache/` — belt-and-suspenders
   population of Claude Code's internal plugin cache.
4. **Registry updates** to `~/.claude/plugins/known_marketplaces.json` and
   `installed_plugins.json` — the state Claude Code maintains after it
   first discovers a marketplace.

## Why

Claude Code bug [#2682](https://github.com/anthropics/claude-code/issues/2682)
prevents MCP servers registered via `mcpServers` in `settings.json` or
`.claude.json` from loading tools reliably. Plugin-registered MCP servers
use a separate, working code path. This library packages the install
procedure as a reusable dependency for Go-based MCP servers that want to
bypass the bug.

## Usage

```go
package main

import (
    "log"

    installer "github.com/suykerbuyk/claude-plugin-installer"
)

func main() {
    id := installer.Identity{
        PluginName: "rezbldr",
        PluginDesc: "MCP server for deterministic resume pipeline operations",
    }.WithDefaults()

    paths, err := installer.Default(id)
    if err != nil {
        log.Fatal(err)
    }

    cfg := installer.Config{
        Version:    "0.2.0",
        BinaryPath: "/home/user/.local/bin/rezbldr",
        ExtraArgs:  []string{"--vault", "/home/user/obsidian/RezBldrVault"},
    }

    if err := installer.Install(paths, cfg); err != nil {
        log.Fatal(err)
    }
}
```

See the [godoc](https://pkg.go.dev/github.com/suykerbuyk/claude-plugin-installer)
for full API reference.

## Opting out of settings manipulation

If the caller wants to manage `~/.claude/settings.json` itself (e.g., to
coordinate with other Claude Code state that lives in the same file), set
`Config.SkipSettings = true`. The library will generate marketplace files
and inject cache state, but will not touch `settings.json`.

## Stability

v0.x releases may contain breaking API changes while the library stabilizes
across its initial consumers (rezbldr, vibe-vault, vibe-palace). v1.0 will
commit to semver.

## License

Dual-licensed under MIT OR Apache-2.0.
