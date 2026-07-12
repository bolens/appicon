# Resolve sources and order

Canonical reference for `$XDG_CONFIG_HOME/appicon/sources.json`.

## Default order

When `sources.json` is missing:

`file` → `overrides` → `xdg` → `svgl`

## Stage types

| Type | Returns path? | Notes |
|------|---------------|--------|
| `file` | yes | Query is an existing non-directory file |
| `overrides` | no | Remaps query via `overrides.json`, then continues |
| `xdg` | yes | FreeDesktop themes / `.desktop` / pixmaps |
| `pack` / `dir` | yes | Local pack directory (`dir` is an alias of `pack`) |
| `svgl` | yes | SVGL (default remote) |
| `simple-icons` | yes | Opt-in jsDelivr Simple Icons CDN |
| `dashboard-icons` | yes | Opt-in jsDelivr dashboard-icons CDN |
| `http-index` | yes | Custom index URL + host allowlist |
| `github` | yes | Opt-in GitHub user/org avatar |
| `glyph` | yes | Opt-in generated monogram SVG (never miss when reached) |

## Compatibility

- Builtins among `{file, overrides, xdg}` that are **not** listed are **prepended** in that relative order (so remote-only configs still get local stages first).
- Top-level flags `"file": false`, `"overrides": false`, `"xdg": false` omit that stage entirely (including if listed).
- Unknown `type` → error (exit `2` / MCP IsError).

## Example

```json
{
  "sources": [
    { "type": "overrides" },
    { "type": "pack", "name": "mine", "path": "~/.local/share/appicon/packs/mine" },
    { "type": "svgl" },
    { "type": "simple-icons" },
    { "type": "dashboard-icons" },
    { "type": "xdg" },
    { "type": "github" },
    { "type": "glyph" }
  ]
}
```

## CLI

```bash
appicon sources list [--json]
appicon sources get [--json]
appicon sources set [--file PATH]   # or JSON on stdin
appicon sources path
appicon resolve --order glyph,svgl,xdg firefox
appicon status
```

`--order` reorders by **type**. Multiple `pack` entries keep their relative config order when `pack` appears once.

### `http-index` example

Custom catalog + host allowlist (do not point at third-party CDNs unless you control them):

```json
{
  "sources": [
    { "type": "overrides" },
    { "type": "xdg" },
    {
      "type": "http-index",
      "name": "homelab",
      "index": "https://icons.example.com/index.json",
      "hosts": ["icons.example.com"]
    },
    { "type": "svgl" }
  ]
}
```

## Offline

`--offline` / `APPICON_OFFLINE=1`: no network. CDN/github/svgl use cache only; `pack install` / `pack update` refuse.

## MCP

| Tool | Role |
|------|------|
| `sources_list` / `sources_get` / `sources_set` | Effective order / raw config / overwrite |
| `resolve` / `prefetch` | Optional `order` array |

Allowlisted CDN hosts (when those stages are enabled): `cdn.jsdelivr.net`; GitHub: `github.com`, `avatars.githubusercontent.com`. Consumers must not embed these URLs — call `appicon` / MCP only.
