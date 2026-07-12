---
name: Standalone appicon CLI
overview: "bolens/appicon — Go CLI resolving desktop/brand icons to local paths. Plan items complete (daemon, AUR refs, cosign); publish AUR when ready."
todos:
  - id: scaffold-repo
    content: Clone bolens/appicon into /home/panda/dev/appicon; scaffold Go module, Makefile, LICENSE, CONTRIBUTING, AGENTS, README; push
    status: completed
  - id: xdg-resolve
    content: Implement XDG/.desktop icon theme resolution with fixture-based unit tests (fake theme + .desktop trees)
    status: completed
  - id: svgl-resolve
    content: Implement SVGL search/download with host allowlist, durable cache, httptest fixtures, rate-limit handling
    status: completed
  - id: png-output
    content: Add --format png|svg and --size; rasterize SVG for Waybar/GTK CSS consumers (cache PNG beside SVG)
    status: completed
  - id: ci-tests
    content: Wire make check / check-fast, golangci-lint, go test, gofmt, gitleaks, actionlint, markdownlint; PR + release workflows
    status: completed
  - id: offline-prune
    content: "--offline + cache prune"
    status: completed
  - id: pluggable-sources
    content: "sources.json — svgl, dir packs, http-index (allowlisted); docs for Simple Icons / dashboard-icons"
    status: completed
  - id: resolve-quality
    content: "Steam/games heuristics + Snap (/var/lib/snapd/desktop) + Flatpak exports"
    status: completed
  - id: release-v0
    content: Tag v0.1.0 with checksums; verify install script against release assets
    status: completed
  - id: waybar-consume
    content: Add install-appicon.sh + make target; dock CSS proof behind settings flag with glyph fallback
    status: completed
  - id: mcp-server
    content: "Post-v1: MCP server wrapping resolve/prefetch/cache so agents can call appicon without shelling out"
    status: completed
  - id: cli-polish
    content: "Post-v1: shell completions (bash/zsh/fish), man page"
    status: completed
  - id: nix-flake
    content: "Post-v1: flake.nix (build + apps.appicon) for nix run / nix profile; same packaging tier as AUR"
    status: completed
  - id: home-manager
    content: "Post-v1: Home Manager module (programs.appicon.enable) pairing with the flake"
    status: completed
  - id: nightly-svgl
    content: "Post-v1: nightly/workflow_dispatch live SVGL smoke (1–2 titles); not required to merge"
    status: completed
  - id: extra-consumers
    content: "Post-v1: Rofi/walker + notification helper examples (shell out to appicon only)"
    status: completed
  - id: daemon-socket
    content: "Optional user systemd socket daemon; resolve dials with in-process fallback"
    status: completed
  - id: release-signing
    content: "Post-v1: optional cosign/sigstore signing beyond SHA256SUMS"
    status: completed
  - id: aur-package
    content: "Post-v1: AUR package (same tier as Nix flake)"
    status: completed
isProject: false
---

# Standalone appicon CLI + Waybar consumer

## Progress (2026-07-12)

**Repo:** [bolens/appicon](https://github.com/bolens/appicon) at `/home/panda/dev/appicon`.

| Area | Status |
|------|--------|
| Scaffold + CI workflows + `make check` / `check-fast` | **Done** |
| XDG / `.desktop` (Flatpak + Snap roots, theme inheritance) | **Done** |
| Steam appid / `steam_icon_*` / `steam://rungameid/` | **Done** |
| SVGL cache-first + allowlist + stale catalog | **Done** |
| Local `dir` packs + `http-index` remotes via `sources.json` | **Done** |
| PNG raster (`resvg`/`rsvg-convert`/oksvg) + raster cache | **Done** |
| `--offline`, `cache prune`/`clear`/`stats`/`path` | **Done** |
| Overrides (`overrides.json`) + CLI `--json` e2e tests | **Done** |
| Tag **`v0.1.0`** + checksummed release assets | **Done** — [v0.1.0](https://github.com/bolens/appicon/releases/tag/v0.1.0) |
| waybar-config install + dock CSS proof | **Done** — `make install-appicon` + `icons.appicon` in waybar-config |
| MCP server (`appicon mcp`) | **Done** — `internal/appmcp` + stdio tools |
| Completions + man (`appicon completion`, `appicon man`) | **Done** |
| Nix flake + Home Manager module | **Done** — `flake.nix`, `nix/home-manager.nix` (set `vendorHash` on first build) |
| Nightly live SVGL smoke | **Done** — `.github/workflows/nightly-svgl.yml` |
| Extra consumer examples | **Done** — `examples/{rofi,walker,notify}-appicon.sh` |
| Optional socket daemon | **Done** — `appicon daemon` + `contrib/systemd/` |
| AUR reference PKGBUILDs | **Done** — `packaging/aur/{appicon,appicon-bin,appicon-git}` (not yet pushed to aur.archlinux.org) |
| Cosign keyless release signing | **Done** — `SHA256SUMS.sigstore.json` on tag releases |

**Packages shipped:** `cmd/appicon`, `internal/resolve`, `internal/xdg`, `internal/svgl`, `internal/pack`, `internal/httpindex`, `internal/cache`, `internal/raster`, `internal/appmcp`, `internal/completion`, `internal/version`.

**Tests:** fixture trees under `testdata/xdg` + `testdata/svgl`; httptest for SVGL/http-index; CLI e2e in `cmd/appicon`; behavioral resolve-order tests in `internal/resolve`; MCP in-memory session tests in `internal/appmcp`. `make check` is the gate.

## Decision

Build **`bolens/appicon`** (new Go repo), **not** `waybar-appicon`. The CLI returns local icon paths — Waybar is the first consumer; the same binary serves Rofi, window-switcher, notifications, etc.

Go: static linux/amd64 + arm64 release binaries (same install pattern as this config’s CI tool downloads).

**Remote:** [bolens/appicon](https://github.com/bolens/appicon). Clone to `/home/panda/dev/appicon`; do not create a new GitHub repo.

## Architecture

```mermaid
flowchart LR
  consumer["Waybar / Rofi / scripts"] -->|"appicon resolve query"| cli[appicon CLI]
  cli --> cache["~/.cache/appicon"]
  cli --> xdg[XDG icon theme + .desktop]
  cli --> sources["sources.json: dir / svgl / http-index"]
  cache -->|"hit"| path[Local SVG or PNG path]
  xdg --> cache
  sources --> cache
  path --> css["CSS background-image or Rofi icons"]
  css --> ui[Consumer UI]
```

**Resolve order (fixed):**

1. Existing file path
2. Freedesktop icon theme (via `.desktop` `Icon=` / name / class / Steam heuristics)
3. Configured logo sources in order (`sources.json`; default: SVGL only)
4. Miss → exit `1` / JSON `"path": null` (callers keep glyphs)

**Cache policy:** network never on hit. Catalog/index TTL ~7d; downloaded assets permanent until `appicon cache clear` or pruned. `--offline` = XDG + disk + local packs only. XDG hits return the theme file path directly (no copy); remote/pack assets live under `$XDG_CACHE_HOME/appicon/` (`svgs/`, `http/`, `raster/`).

## Gaps folded into v1 (previously missing)

| Gap | Plan | Status |
|-----|------|--------|
| **GTK/Waybar SVG CSS** | Default Waybar path: `resolve --format png --size 24`. | **Done** |
| **Size / theme** | `--size N`, `--theme dark\|light`, `APPICON_THEME` / icon-theme override. | **Done** |
| **Name mismatches** | `overrides.json` + small built-in aliases. | **Done** |
| **SSRF / safety** | SVGL hosts allowlisted; http-index requires per-source `hosts`. | **Done** |
| **Rate limits** | Cache-first; 429/5xx → stale catalog; ~2.5s timeout. | **Done** |
| **Atomic cache** | `.tmp` + rename; flock on catalog refresh. | **Done** |
| **License / brands** | MIT for code only; no logo packs in releases. | **Done** (docs) |
| **Install integrity** | Release `SHA256SUMS`; waybar install script verifies. | Pending release |
| **Flatpak / Snap** | Flatpak export dirs + `/var/lib/snapd/desktop`. | **Done** |
| **Steam** | appid / WM class / Exec `steam://rungameid/`. | **Done** |
| **Docs for agents** | `AGENTS.md` + `CONTRIBUTING.md`. | **Done** |

## CLI surface

| Command | Behavior | Status |
|---------|----------|--------|
| `appicon resolve <query>` | Print absolute path | **Done** |
| `appicon resolve --json` | Structured result (`source`, `theme`, `cached`, `format`, …) | **Done** |
| `appicon resolve --format png\|svg --size N --theme dark\|light` | Output format / variant | **Done** |
| `appicon resolve --offline` | No network | **Done** |
| `appicon prefetch <query>...` | Warm cache | **Done** |
| `appicon cache path\|clear\|stats\|prune` | Cache management | **Done** |
| `appicon daemon [\-\-socket PATH]` | Optional unix-socket resolve server | **Done** |
| `appicon mcp` | Stdio MCP tools (resolve/prefetch/cache/version) | **Done** |
| `appicon completion bash\|zsh\|fish` | Print shell completion script | **Done** |
| `appicon man` | Print embedded man page | **Done** |
| `appicon version` | Semver from release ldflags | **Done** |

**Query inputs:** app id, WM class, `foo.desktop`, display name, Steam appid, or filesystem path.

**Config:**

- `$XDG_CONFIG_HOME/appicon/overrides.json` — query remaps
- `$XDG_CONFIG_HOME/appicon/sources.json` — ordered `svgl` / `dir` / `http-index`

## Tests and CI

**Local gates:** `make check-fast` (test + vet + gofmt), `make check` (+ golangci-lint when present, gitleaks, actionlint, markdownlint).

**Coverage in tree:** XDG fixtures (incl. Flatpak/Snap/Steam), SVGL httptest, http-index httptest, pack unit tests, resolve behavioral order, CLI e2e (`cmd/appicon`).

**GitHub Actions:** `ci.yml` on PR/push; `release.yml` on tag `v*` (amd64/arm64 + `SHA256SUMS`).

## Waybar-config consumer (after first release)

1. `scripts/infra/install-appicon.sh` — pin `APPICON_VERSION`, download + checksum verify → `~/.local/bin/appicon` (or `$WAYBAR_HOME/bin/`).
2. `make install-appicon` + README/scripts note.
3. Proof integration: dock launcher only — prefetch/resolve to PNG, generate CSS (`#custom-dock-* { background-image: url(...); }`), glyph fallback if binary missing or resolve fails.
4. Settings: `icons.appicon.enabled` / `theme` / `size` in waybar-settings; no-op when binary absent.

No SVGL URLs inside waybar scripts — only `appicon resolve`.

## Explicitly out of scope (deferred)

These stay **out of the product roadmap unless a concrete consumer forces them**. Notes below are enough to restart the design without rediscovering constraints.

### Daemon / socket

**Status:** **Done** (optional; not required for Waybar dock).

- User systemd: `contrib/systemd/appicon.socket` + `appicon.service` (socket activation via `LISTEN_FDS`)
- Protocol: big-endian uint32 length + JSON (`op=resolve|ping`) mirroring `resolve --json`
- Transport: `$XDG_RUNTIME_DIR/appicon.sock` (mode `0600`); `APPICON_SOCKET` override; abstract sockets rejected
- `appicon resolve` dials when socket present; `--local` / `APPICON_NO_DAEMON=1` skips; dial failure falls back in-process
- Same `internal/resolve` path — no second cache or download allowlist

### Replacing tray SNI icons

**Why deferred:** StatusNotifierItem / KDE tray / AppIndicator icons are owned by the application process and compositor/panel. Swapping them means either LD_PRELOAD/hacks, a custom tray host, or patching each app — far outside “return a file path.”

**If revisited:**

- Scope as a **separate tool** (e.g. `appicon-trayd`) that only helps *our* panels (Waybar custom modules), not system-wide SNI replacement.
- Read icons for *display* via `appicon resolve` (PNG); never claim to be an SNI host unless implementing the full StatusNotifierWatcher/Item D-Bus APIs.
- Document that Electron/Chromium tray icons and proprietary indicators will remain opaque.

### Full logo catalogs vendored into release tarballs

**Why deferred:** Brand marks are third-party; shipping a logo pack in GitHub Releases creates redistribution / trademark / size problems and fights the “code only, MIT” stance. SVGL’s catalog is large and changes upstream.

**If revisited:**

- Prefer **user-cloned `dir` packs** (Simple Icons, dashboard-icons) or `http-index` with explicit hosts — already supported.
- Optional “offline bundle” would be a **separate artifact** (not the default `appicon_*_linux_*.tar.gz`), with its own license file and pin to an upstream commit SHA.
- Never merge a full catalog into `go:embed` or the main binary; keep releases tiny and static.

### `appicon self-update`

**Why deferred:** One more updater channel to secure (signature, rollback, partial downloads). We already plan checksummed GitHub releases + waybar `install-appicon.sh`, plus later Nix/AUR/Home Manager.

**Instead:**

- Re-run the install script / `make install-appicon` with a bumped pin.
- Or upgrade via package manager once AUR/Nix exist.
- If a self-update subcommand appears later: download release asset + `SHA256SUMS` (and cosign if enabled), verify, atomic replace under the same path as the running binary; refuse to update when installed via Nix/pacman (detect read-only store / package manager metadata).

### Perfect coverage for obscure apps

**Why deferred:** Infinite tail — proprietary WM classes, broken `.desktop` files, games without shortcuts, renamed Electron apps. Chasing 100% match rate balloons aliases and special cases.

**Instead:**

- Overrides file + Steam/Flatpak/Snap heuristics (done) cover the common miss classes.
- Users add `dir` packs / `http-index` / `overrides.json` for personal long-tail apps.
- Accept miss → exit `1` / glyph fallback as a **supported** outcome, not a bug.
- New heuristics only when a recurring miss shows up in a real consumer (e.g. waybar dock list), with a fixture test — no speculative alias dumps.

## Post-v1

Follow-ups after a tagged release + Waybar proof.

### Packaging / install

| Item | Notes | Status |
|------|-------|--------|
| **Completions + man** | `appicon completion` / `appicon man`; scripts + man1 in release tarball | **Done** |
| **Nix flake** | `flake.nix`: package + `apps.appicon` for `nix run` | **Done** (update `vendorHash`) |
| **Home Manager** | `programs.appicon.enable` via `homeManagerModules.default` | **Done** |
| **AUR** | Reference PKGBUILDs: `appicon`, `appicon-bin`, `appicon-git` | **Done** (publish manually) |
| **Release signing** | Cosign keyless OIDC → `SHA256SUMS.sigstore.json` | **Done** |

### Pluggable logo sources

**Done:** `sources.json` with `svgl`, `dir`, `http-index` (per-source host allowlist).

**Docs:** clone Simple Icons / dashboard-icons locally; do not bake CDNs into the binary. Example dock-oriented order:

```text
path → XDG → dir packs (user) → svgl → miss
```

```json
{
  "sources": [
    { "type": "dir", "path": "~/.local/share/appicon/packs/dashboard-icons" },
    { "type": "dir", "path": "~/.local/share/appicon/packs/simple-icons/icons" },
    { "type": "svgl" },
    {
      "type": "http-index",
      "name": "my-cdn",
      "index": "https://icons.example/index.json",
      "hosts": ["icons.example"]
    }
  ]
}
```

**Do not promote:** Logo.dev / Brandfetch / Clearbit (API keys), Iconify (host/license surface), vendoring packs in releases.

### Extra consumers / CI

- **Done:** `examples/rofi-appicon.sh`, `examples/walker-appicon.sh`, `examples/notify-appicon.sh` — shell-out only
- **Done:** Nightly / `workflow_dispatch` live SVGL smoke (`.github/workflows/nightly-svgl.yml`)

### MCP server (agent tooling)

**Status:** **Done** — `appicon mcp` via `internal/appmcp` (official Go MCP SDK, stdio).

**Tools (map 1:1 to CLI; no new resolve logic):**

| Tool | Mirrors | Notes |
|------|---------|-------|
| `resolve` | `appicon resolve --json` | args: `query`, optional `format`, `size`, `theme`, `offline`; returns path/source/cached/error |
| `prefetch` | `appicon prefetch` | args: `queries[]` |
| `cache_stats` | `appicon cache stats` | |
| `cache_clear` / `cache_prune` | matching subcommands | destructive — documented in tool descriptions |
| `version` | `appicon version` | |

**Rules:**

- Call into `internal/resolve` (same as CLI) — **never** reimplement download/allowlist in the MCP layer.
- Still no SVGL URLs in agent prompts or other repos; agents call `resolve` only.
- Example Cursor/Claude config in README (`command`: `appicon`, `args`: `["mcp"]`).
- Tests: in-memory MCP session tests with fixture XDG roots (`internal/appmcp`); no live network required.

**Out of scope for the MCP:** browsing remote catalogs in-agent, writing `overrides.json` without an explicit tool, or exposing raw HTTP downloads.

## Execution order

1. ~~Clone + scaffold~~
2. ~~XDG resolve + fixtures~~
3. ~~Cache + SVGL + httptest + allowlist~~
4. ~~PNG output~~
5. ~~CI workflows + `make check`~~
6. ~~`--offline`, prune, packs, http-index, Steam, Snap~~
7. ~~Cut `v0.1.0` with checksums~~
8. ~~waybar-config install + dock CSS proof~~
9. ~~MCP server for agents~~
10. ~~Completions/man~~
11. ~~Nix / Home Manager; nightly SVGL; extra consumer examples~~
12. ~~Optional socket daemon~~
13. ~~AUR reference PKGBUILDs; cosign keyless release signing~~
14. Manual: push AUR packages to aur.archlinux.org; cut next release to publish cosign bundles
