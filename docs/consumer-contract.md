# Consumer contract

Stable interface for Waybar, Rofi, scripts, and agents. Treat misses as **supported** outcomes (keep glyphs / hide chrome), not bugs.

## Exit codes (`resolve`)

| Code | Meaning | Typical consumer action |
|------|---------|-------------------------|
| `0` | Icon found; path printed (or JSON `path` set) | Use the path |
| `1` | Not found (`resolve.ErrNotFound`) | Glyph / omit; do not retry hot-path without cache warm |
| `2` | Usage / I/O / unexpected error | Log once; degrade like a miss |

`--json` always writes one JSON object to stdout **before** exiting non-zero on a miss or resolve error.

## `resolve --json` schema

Stable keys (do not rename):

| Field | Type | Notes |
|-------|------|-------|
| `query` | string | Echo of the request |
| `path` | string \| `null` | Absolute local file path on success |
| `source` | string | `file` \| `xdg` \| `svgl` \| `pack` \| `http-index` \| `""` on miss |
| `theme` | string | Effective theme hint |
| `format` | string | `svg` \| `png` |
| `cached` | bool | Whether the hit came from appicon’s durable cache |
| `error` | string \| `null` | Set when `path` is `null` |

Plain (non-JSON) mode: success prints one path line; miss prints nothing on stdout and exits `1`.

## Offline / daemon

- `--offline` — XDG + local packs + on-disk cache only; never opens the network.
- Hot paths (e.g. Waybar dock ticks) should use `--offline` after a one-shot online `prefetch`.
- `resolve` dials `$XDG_RUNTIME_DIR/appicon.sock` when present; falls back in-process. `--local` / `APPICON_NO_DAEMON=1` skips the dial.

## Optional peer (Waybar-style)

Like `zscroll` / `cava`: install is optional. Consumers must:

1. Keep a non-appicon fallback (glyph, CSS without `background-image`, hide module).
2. Avoid restart-spam or PATH probes every signal when the binary is missing (negative-cache until bar reload).
3. Never embed SVGL (or other) URLs — only shell out to `appicon`.

- Optional remaps: `$XDG_CONFIG_HOME/appicon/overrides.json` via `appicon override` / MCP `override_*`.
