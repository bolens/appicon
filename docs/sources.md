# Resolve sources and order

Canonical reference for `$XDG_CONFIG_HOME/appicon/sources.json` **or** `sources.yaml` / `sources.yml` (same for `overrides`).

Schema: [sources.schema.json](sources.schema.json). Packs: [packs.md](packs.md). Consumer exits/JSON: [consumer-contract.md](consumer-contract.md).

## Config formats

- Prefer **one** of `sources.json`, `sources.yaml`, or `sources.yml`. Having more than one is an error.
- Same rule for `overrides.json` / `overrides.yaml` / `overrides.yml`.
- `appicon sources set [--format json|yaml]` accepts JSON or YAML on stdin/`--file`. When no file exists yet, `--format` picks the extension (default JSON). When a file already exists, writes keep that format.

## Default order

When sources config is missing:

`file` → `overrides` → `xdg` → `svgl`

## Stage types

| Type | Returns path? | Notes |
|------|---------------|--------|
| `file` | yes | Query is an existing non-directory file |
| `overrides` | no | Remaps query via overrides config, then continues |
| `xdg` | yes | FreeDesktop themes / `.desktop` / pixmaps |
| `pack` / `dir` | yes | Local pack directory (`dir` is an alias of `pack`) |
| `svgl` | yes | SVGL (default remote) |
| `simple-icons` | yes | Opt-in jsDelivr Simple Icons CDN |
| `dashboard-icons` | yes | Opt-in jsDelivr dashboard-icons CDN |
| `http-index` | yes | Custom index URL + host allowlist; optional `token_env` → Bearer |
| `github` | yes | Opt-in GitHub avatar and/or repo Contents API (optional PAT) |
| `logo-dev` | yes | Opt-in Logo.dev brand logos; requires `token_env` |
| `iconify` | yes | Opt-in Iconify API (`prefix:name`); optional `base` for self-host |
| `noun-project` | yes | Opt-in Noun Project; requires `token_env` + `secret_env` (OAuth1) |
| `glyph` | yes | Opt-in generated monogram SVG (never miss when reached) |

## BYOK credentials

Secrets are **never** stored in sources/overrides files. Use env var **names** only:

| Field | Meaning |
|-------|---------|
| `token_env` | Name of env var holding API token / OAuth1 consumer key / Logo.dev publishable key / GitHub PAT |
| `secret_env` | Name of env var holding OAuth1 consumer secret (`noun-project` only) |

If `token_env` / `secret_env` is set on a stage but the env var is missing/empty, that stage is **skipped** (benign miss). With `--explain`, `tried` labels it as `stage(auth)` (e.g. `logo-dev(auth)`, `noun-project(auth)`, `github(auth)`). Check readiness with `appicon status` → `credentials`.

```yaml
sources:
  - type: overrides
  - type: xdg
  - type: logo-dev
    token_env: LOGO_DEV_TOKEN
  - type: noun-project
    token_env: NOUN_PROJECT_KEY
    secret_env: NOUN_PROJECT_SECRET
  - type: github
    token_env: GITHUB_TOKEN
    path: myorg/private-icons   # optional default repo; query = file stem
  - type: iconify
  - type: svgl
```

### Provider query notes

- **logo-dev:** domain-like query (`shopify.com`). Allowlist: `img.logo.dev`.
- **iconify:** `prefix:name` (e.g. `mdi:firefox`). Default base `https://api.iconify.design`.
- **noun-project:** numeric icon id or search term (public-domain search). Allowlist: `api.thenounproject.com`. Quota is on **your** API key.
- **github:**
  - Owner / `https://github.com/owner` → avatar (`github.com` / `avatars.githubusercontent.com`; with PAT also `api.github.com`)
  - `owner/repo/path/to/icon.svg` or blob URL → Contents API (requires PAT); raw accept stays on `api.github.com`
  - Stage `path: owner/repo` + stem query tries `{stem}.svg` then `.png`
  - Minimal PAT scopes: `read:user` (avatars), `repo` (private contents)

## Platform notes

Primary target is **Linux** (FreeDesktop XDG, Flatpak/Snap data roots, optional systemd user daemon).

| Surface | Notes |
|---------|--------|
| Config / cache / packs | Honor `XDG_*` when set; otherwise `~/.config`, `~/.cache`, `~/.local/share` on Unix, and OS user dirs on Windows (`UserConfigDir` / `UserCacheDir` / `%LOCALAPPDATA%` for packs) |
| XDG icon lookup | Flatpak/Snap/`/usr` defaults are Linux-only |
| Daemon | Unix socket only; `status.daemon_supported` is false on Windows — resolve stays in-process |
| Home Manager | Linux/systemd; use `environmentFiles` for BYOK secret *values* (never `sops.secrets.*.path` in `environment`) |

## Compatibility

- Builtins among `{file, overrides, xdg}` that are **not** listed are **prepended** in that relative order (so remote-only configs still get local stages first).
- Top-level flags `"file": false`, `"overrides": false`, `"xdg": false` omit that stage entirely (including if listed).
- Unknown `type` → error (exit `2` / MCP IsError).

## Example (JSON)

```json
{
  "sources": [
    { "type": "overrides" },
    { "type": "pack", "name": "mine", "path": "~/.local/share/appicon/packs/mine" },
    { "type": "svgl" },
    { "type": "simple-icons" },
    { "type": "dashboard-icons" },
    { "type": "xdg" },
    { "type": "github", "token_env": "GITHUB_TOKEN" },
    { "type": "glyph" }
  ]
}
```

## CLI

```bash
appicon sources list [--json]
appicon sources get [--json]
appicon sources set [--file PATH] [--format json|yaml]
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
      "hosts": ["icons.example.com"],
      "token_env": "HOMELAB_ICONS_TOKEN"
    },
    { "type": "svgl" }
  ]
}
```

## Offline

`--offline` / `APPICON_OFFLINE=1`: no network. CDN/github/svgl/logo-dev/iconify/noun-project use cache only; `pack install` / `pack update` refuse.

## Theme (color scheme)

`--theme dark|light`, `APPICON_THEME`, or `GTK_THEME` suffix (`Adwaita:dark`) prefer matching SVGL/CDN/logo-dev variants and XDG names (`name-dark`, `name-symbolic`, `name-light`). FreeDesktop **icon theme name** is separate (`APPICON_ICON_THEME` / `GTK_THEME` basename before `:`).

## MCP

| Tool | Role |
|------|------|
| `sources_list` / `sources_get` / `sources_set` | Effective order / raw config / overwrite |
| `resolve` | Optional `order`, `theme`, `explain`; `query` or `queries` (batch → `{results:[…]}`) |
| `prefetch` | Optional `order`, `theme`, `offline`, `from_desktop` |
| `override_suggest` / `override_export` / `override_import` | Remap suggestions; bulk JSON/YAML dump/load |
| `status` | Paths, order, `credentials` (BYOK readiness), `daemon_alive`, `daemon_supported`, `goos`/`goarch` |

Allowlisted CDN hosts (when those stages are enabled): `cdn.jsdelivr.net`; GitHub: `github.com`, `avatars.githubusercontent.com`, `api.github.com`; Logo.dev: `img.logo.dev`; Iconify: host from `base` (default `api.iconify.design`); Noun Project: `api.thenounproject.com`. Consumers must not embed these URLs — call `appicon` / MCP only.

## See also

- [Documentation map](README.md)
- [packs.md](packs.md) — local packs vs CDN stages
- [consumer-contract.md](consumer-contract.md) — exit codes / `--json` (single + batch)
- [deferred.md](deferred.md) — not a backlog
- [../README.md](../README.md) · [../SECURITY.md](../SECURITY.md) · [../AGENTS.md](../AGENTS.md)
