# Consumer contract

Stable interface for Waybar, Rofi, scripts, and agents. Treat misses as **supported** outcomes (keep glyphs / hide chrome), not bugs.

Part of the [documentation map](README.md). Machine schemas: [resolve-result.schema.json](resolve-result.schema.json), [resolve-batch-result.schema.json](resolve-batch-result.schema.json). Stages: [sources.md](sources.md).

## Exit codes (`resolve`)

| Code | Meaning | Typical consumer action |
|------|---------|-------------------------|
| `0` | Icon found; path printed (or JSON `path` set) | Use the path |
| `1` | Not found (`resolve.ErrNotFound`) — for multi-query, any miss | Glyph / omit; do not retry hot-path without cache warm |
| `2` | Usage / I/O / unexpected error | Log once; degrade like a miss |

`--json` always writes JSON to stdout **before** exiting non-zero on a miss or resolve error.

## `resolve --json` schema

### Single query

Stable keys (do not rename):

| Field | Type | Notes |
|-------|------|-------|
| `query` | string | Echo of the request |
| `path` | string \| `null` | Absolute local file path on success |
| `source` | string | `file` \| `xdg` \| `svgl` \| `pack` \| `http-index` \| `simple-icons` \| `dashboard-icons` \| `github` \| `logo-dev` \| `iconify` \| `noun-project` \| `glyph` \| `""` on miss |
| `theme` | string | Effective theme hint (`dark`/`light`/`""`; from `--theme`, `APPICON_THEME`, or `GTK_THEME` `:dark`/`:light`) |
| `format` | string | `svg` \| `png` |
| `cached` | bool | Whether the hit came from appicon’s durable cache |
| `error` | string \| `null` | Set when `path` is `null` |
| `tried` | string[] (optional) | With `--explain`: stage labels that missed before the hit or final miss. Auth-skipped BYOK stages use `stage(auth)` (e.g. `logo-dev(auth)`). |
| `hint` | string (optional) | With `--explain` on miss: actionable next steps |

Machine-readable schema: [resolve-result.schema.json](resolve-result.schema.json).

### Multiple queries (batch)

`appicon resolve --json q1 q2 …` emits:

```json
{ "results": [ /* same object shape as single query, one per input */ ] }
```

Machine schema: [resolve-batch-result.schema.json](resolve-batch-result.schema.json) (items reuse [resolve-result.schema.json](resolve-result.schema.json)).

Plain (non-JSON) mode: one path line per hit; misses print hints on stderr and contribute to exit `1`.

## Offline / daemon

- `--offline` — XDG + local packs + on-disk cache only; never opens the network.
- Hot paths (e.g. Waybar dock ticks) should use `--offline` after a one-shot online `prefetch` (optionally `prefetch --from-desktop`).
- `resolve` / `prefetch` dial `$XDG_RUNTIME_DIR/appicon.sock` when present; fall back in-process. `--local` / `APPICON_NO_DAEMON=1` skip the dial.
- Daemon is Unix-only (`status.daemon_supported`); Windows always resolves in-process.
- Daemon frames carry `order`, `explain`, miss `hint`, and `resolve-batch` (`queries`) — same allowlists/cache as the CLI.
- `status` may include `credentials` (BYOK env readiness) and platform fields (`goos`/`goarch`).

## Theme

`--theme dark|light` prefers matching SVGL/CDN variants and XDG icon names (`name-dark`, `name-symbolic`, `name-light`). When unset, `APPICON_THEME` then `GTK_THEME` suffix (`Adwaita:dark`) apply. FreeDesktop **icon theme name** remains `APPICON_ICON_THEME` / `GTK_THEME` basename (before `:`).

## Overrides / suggest

Long-tail remaps: `appicon override set|get|list|rm|export|import`. After misses, `appicon override suggest <query>` (or `--from-misses`) proposes candidates from `.desktop` Icon=, catalog, and existing overrides — never speculative aliases in code.

## Consumer quickstart (Waybar-style)

```bash
# One-shot warm (online)
appicon prefetch firefox discord code
# or: appicon prefetch --from-desktop

# Hot path (no network); batch when resolving a dock list
path=$(appicon resolve --offline firefox) || true
appicon resolve --json --offline firefox discord code
# miss → exit 1; keep glyph / omit module
```

Inspect install health: `appicon status`.

Offline CI-style smoke: `make check-consumer-smoke` ([scripts/ci/consumer-smoke.sh](../scripts/ci/consumer-smoke.sh)) validates single + batch JSON against the schemas and `bash -n` on `examples/*.sh`.

## Optional peer (Waybar-style)

Like `zscroll` / `cava`: install is optional. Consumers must:

1. Keep a non-appicon fallback (glyph, CSS without `background-image`, hide module).
2. Avoid restart-spam or PATH probes every signal when the binary is missing (negative-cache until bar reload).
3. Never embed SVGL (or other) URLs — only shell out to `appicon`.

- Optional remaps: `$XDG_CONFIG_HOME/appicon/overrides.json` via `appicon override` / MCP `override_*`.
- Resolve stages / packs: [sources.md](sources.md), [packs.md](packs.md). Enabling `glyph` as a stage yields exit `0` with `source: glyph` instead of a miss.

## See also

- [Documentation map](README.md)
- [resolve-result.schema.json](resolve-result.schema.json) · [resolve-batch-result.schema.json](resolve-batch-result.schema.json)
- [sources.md](sources.md) · [packs.md](packs.md) · [deferred.md](deferred.md)
- [../README.md](../README.md) · [../AGENTS.md](../AGENTS.md) · [../SECURITY.md](../SECURITY.md)
