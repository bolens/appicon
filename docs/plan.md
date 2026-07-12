---
name: Standalone appicon CLI
overview: Create bolens/appicon — a Go CLI that resolves desktop icons (XDG first, SVGL cached fallback), with waybar-grade tests/CI, signed-style release installs, and a thin waybar-config consumer using CSS background-image (PNG-safe for GTK).
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
  - id: release-v0
    content: Tag v0.1.0 with checksums; verify install script against release assets
    status: pending
  - id: waybar-consume
    content: Add install-appicon.sh + make target; dock CSS proof behind settings flag with glyph fallback
    status: pending
  - id: nix-flake
    content: "Post-v1: flake.nix (build + apps.appicon) for nix run / nix profile; same packaging tier as AUR"
    status: pending
  - id: home-manager
    content: "Post-v1: Home Manager module (programs.appicon.enable) pairing with the flake"
    status: pending
  - id: cli-polish
    content: "Post-v1: shell completions (bash/zsh/fish), man page (--offline + cache prune done)"
    status: pending
  - id: pluggable-sources
    content: "Post-v1: http-index + docs for Simple Icons / dashboard-icons dir packs (dir + sources.json done)"
    status: pending
  - id: nightly-svgl
    content: "Post-v1: nightly/workflow_dispatch live SVGL smoke (1–2 titles); not required to merge"
    status: pending
  - id: release-signing
    content: "Post-v1: optional cosign/sigstore signing beyond SHA256SUMS"
    status: pending
isProject: false
---

# Standalone appicon CLI + Waybar consumer

## Decision

Build **`bolens/appicon`** (new Go repo), **not** `waybar-appicon`. The CLI returns local icon paths — Waybar is the first consumer; the same binary serves Rofi, window-switcher, notifications, etc.

Go: static linux/amd64 + arm64 release binaries (same install pattern as this config’s CI tool downloads).

**Remote already exists:** [bolens/appicon](https://github.com/bolens/appicon) (empty). Clone to `/home/panda/dev/appicon`, scaffold, push to `main` — do not create a new GitHub repo.

## Architecture

```mermaid
flowchart LR
  consumer["Waybar / Rofi / scripts"] -->|"appicon resolve query"| cli[appicon CLI]
  cli --> cache["~/.cache/appicon"]
  cli --> xdg[XDG icon theme + .desktop]
  cli --> sources["Remote / pack sources (SVGL first)"]
  cache -->|"hit"| path[Local SVG or PNG path]
  xdg --> cache
  sources --> cache
  path --> css["CSS background-image or Rofi icons"]
  css --> ui[Consumer UI]
```

**Resolve order (fixed):**

1. Existing file path
2. Freedesktop icon theme (via `.desktop` `Icon=` / name / class)
3. Configured logo sources in order (v1: SVGL only; post-v1: packs + extra remotes)
4. Miss → exit `1` / JSON `"path": null` (callers keep glyphs)

**Cache policy:** network never on hit. Catalog TTL ~7d; downloaded assets permanent until `appicon cache clear` (post-v1: also `cache prune`). Offline = XDG + disk only. XDG hits return the theme file path directly (no copy); only remote/pack assets are written under `$XDG_CACHE_HOME/appicon/`.

## Gaps folded into v1 (previously missing)

| Gap | Plan |
|-----|------|
| **GTK/Waybar SVG CSS** | GTK3 `background-image` is unreliable with SVG. Default Waybar path: `resolve --format png --size 24` (rasterize + cache PNG). Keep SVG for non-GTK consumers. |
| **Size / theme** | `--size N`, `--theme dark\|light`, optional `APPICON_THEME` / icon-theme name override. |
| **Name mismatches** | Optional user override map: `$XDG_CONFIG_HOME/appicon/overrides.json` (`{"zen-browser":"Firefox"}` style) plus a few built-in aliases. |
| **SSRF / safety** | Download only from allowlisted hosts (`api.svgl.app`, `svgl.app`). Reject other redirect targets. |
| **Rate limits** | Cache-first; on HTTP 429/5xx backoff and serve stale catalog if present; never block callers longer than a short timeout (~2–3s). |
| **Atomic cache** | Write `.tmp` + rename; simple file lock for catalog refresh. |
| **License / brands** | MIT (or Apache-2.0) for **code only**. README: SVGL/brand logos are third-party marks — cache locally, do not republish a logo pack in releases. |
| **Install integrity** | Release assets + `SHA256SUMS`; waybar `install-appicon.sh` verifies checksum. |
| **Flatpak** | Resolve `.desktop` under Flatpak export dirs (same idea as waybar `xdg-applications.sh`). Snap: post-v1 best-effort. |
| **Docs for agents** | Repo gets `AGENTS.md` + `CONTRIBUTING.md` (check gates, no secret commits). |

## CLI surface

| Command | Behavior |
|---------|----------|
| `appicon resolve <query>` | Print absolute path |
| `appicon resolve --json <query>` | Structured result incl. `source`, `theme`, `cached`, `format` |
| `appicon resolve --format png\|svg --size N --theme dark\|light` | Output format / variant |
| `appicon prefetch <query>...` | Warm cache |
| `appicon cache path\|clear\|stats` | Cache management |
| `appicon version` | Semver from release ldflags |

**Query inputs:** app id, WM class, `foo.desktop`, display name, or filesystem path.

**Packages:** `cmd/appicon`, `internal/xdg`, `internal/svgl`, `internal/cache`, `internal/raster` (SVG→PNG; prefer a pure-Go lib or optional `rsvg-convert`/`resvg` if present — pick one approach in implementation and document the dependency).

## Tests and CI (waybar-grade, Go-shaped)

Mirror the **discipline** of [waybar-config](https://github.com/bolens/waybar-config) (`make check` / `check-fast`, gitleaks, actionlint, markdownlint), not shell suite matrices.

**Local gates (`Makefile`):**

- `make check-fast` — `go test ./...` (short), `go vet`, `gofmt -l` clean
- `make check` — check-fast + `golangci-lint` + race tests where cheap + gitleaks + markdownlint + actionlint on workflows
- `make test` / `make lint` / `make fmt`

**Test types:**

- **Unit:** XDG resolver against checked-in fixture trees (`testdata/xdg/...` fake `apps/*.desktop` + `icons/hicolor/...`)
- **Unit:** SVGL client against `httptest` + recorded JSON/SVG fixtures (no live network in CI)
- **Unit:** cache TTL, atomic write, host allowlist rejection
- **Unit:** override map + resolve order
- **CLI smoke:** `go test` / small exec tests for `resolve --json` exit codes
- **Optional integration job** (`workflow_dispatch` or nightly): live SVGL resolve of 1–2 known titles; not required to merge

**GitHub Actions:**

- `ci.yml` on PR/push — check-fast + golangci-lint + gitleaks + actionlint + markdownlint
- `release.yml` on tag `v*` — build amd64/arm64, `SHA256SUMS`, GitHub Release
- Pin tool versions (golangci-lint, gitleaks) like waybar pins shfmt/gitleaks

**Repo hygiene:** `.gitignore`, LICENSE, CONTRIBUTING, AGENTS.md, CODEOWNERS optional, `go.mod` with Go 1.22+.

## Waybar-config consumer (after first release)

1. `scripts/infra/install-appicon.sh` — pin `APPICON_VERSION`, download + checksum verify → `~/.local/bin/appicon` (or `$WAYBAR_HOME/bin/`).
2. `make install-appicon` + README/scripts note.
3. Proof integration: dock launcher only — prefetch/resolve to PNG, generate CSS (`#custom-dock-* { background-image: url(...); }`), glyph fallback if binary missing or resolve fails.
4. Settings: `icons.appicon.enabled` / `theme` / `size` in [data/waybar-settings.jsonc](data/waybar-settings.jsonc); no-op when binary absent.

No SVGL URLs inside waybar scripts — only `appicon resolve`.

## Out of scope for v1 (still)

- Daemon/socket
- Replacing tray SNI icons
- Full logo catalogs vendored into release tarballs
- `appicon self-update` (re-run install script / package manager instead)
- Perfect coverage for obscure apps

## Post-v1

Follow-ups after a tagged release + Waybar proof. Same packaging tier as each other unless noted.

### Packaging / install

| Item | Notes |
|------|-------|
| **Nix flake** | `flake.nix`: package + `apps.appicon` for `nix run github:bolens/appicon` |
| **Home Manager** | `programs.appicon.enable` (pairs with flake) |
| **AUR** | Same tier as flake — optional beside GitHub release tarballs |
| **Release signing** | Optional cosign/sigstore in addition to `SHA256SUMS` |
| **Completions + man** | bash/zsh/fish completions; short man page |

### CLI / cache

| Item | Notes |
|------|-------|
| **`--offline`** | **Done** — never touch the network; XDG + on-disk cache + local packs only |
| **`cache prune`** | **Done** — drop regenerable `raster/` and SVGL assets not in catalog; keep `cache clear` as wipe-all |

### Resolve quality

| Item | Notes |
|------|-------|
| **Steam / games** | Heuristics for Steam shortcuts / `steam_appid` / common game `.desktop` misses |
| **Snap exports** | Best-effort `.desktop` + icons under Snap export dirs (like Flatpak) |

### Pluggable logo sources (supplement SVGL)

**Done so far:** ordered `sources.json` with `svgl` + local `dir` packs; default remains SVGL-only.

**Still open:** `http-index` remote providers with per-source host allowlists (see sketch below). Document recommended packs; no new built-in logo APIs.

**Recommended supplements (priority):**

| Priority | Option | Notes |
|----------|--------|-------|
| **1** | **`http-index`** | Generic allowlisted remotes (self-hosted indexes, pinned CDNs, private brand packs). Same timeout + stale-index behavior as SVGL. Primary post-v1 implementation work. |
| **2** | **[Simple Icons](https://github.com/simple-icons/simple-icons) as a documented `dir` pack** | Largest monochrome brand set; clean license story. User clones locally — do **not** bake `cdn.simpleicons.org` / jsDelivr into the binary. |
| **3** | **[dashboard-icons](https://github.com/homarr-labs/dashboard-icons) as a documented `dir` pack** | Strong for docks / self-hosted apps where SVGL + XDG often miss. Same clone → `type: dir` pattern. |

**Lower priority / later:**

- Optional first-class `simple-icons` source type — only if plain `dir` + slug lookup feels awkward; prefer keeping the surface small.
- Other packs (e.g. Devicon) — same `dir` recipe; no new source types until needed.

**Do not add / do not promote:**

- Built-in Logo.dev / Brandfetch / Clearbit (API keys, ToS).
- Built-in Iconify (huge host/license surface).
- Shipping any third-party pack in GitHub releases.
- A second hard-coded remote like SVGL unless license + allowlist are as clean as SVGL — preference stays **user-configured** sources.

**Suggested dock-oriented order** (document as an example; default stays SVGL-only until the user edits config):

```text
path → XDG → dir packs (user) → svgl → miss
```

Local packs often beat SVGL for Waybar docks; brand-first users can put `svgl` before `dir`.

**Config sketch** (`$XDG_CONFIG_HOME/appicon/sources.json`):

```json
{
  "sources": [
    { "type": "dir", "path": "~/.local/share/appicon/packs/dashboard-icons" },
    { "type": "dir", "path": "~/.local/share/appicon/packs/simple-icons" },
    { "type": "svgl", "enabled": true },
    {
      "type": "http-index",
      "name": "my-cdn",
      "index": "https://icons.example/index.json",
      "hosts": ["icons.example"]
    }
  ]
}
```

**Rules:**

- Resolve still runs **path → XDG → sources (in order) → miss**.
- **Local `dir` packs:** tree or flat folder of `.svg`/`.png` plus optional `index.json` (`{"Firefox":"firefox.svg"}`); no network.
- **Remote providers:** must declare an **explicit host allowlist** (extend today’s `api.svgl.app` / `svgl.app` gate). Reject redirects off-allowlist. Same short timeout + stale-index behavior as SVGL.
- **Do not** ship third-party logo packs inside GitHub releases; document how to clone Simple Icons, dashboard-icons, or a custom brand folder and point `sources.json` at them.
- Brand/trademark disclaimer stays: cache for personal use; appicon distributes code only.
- Built-in second remote only if license + allowlist story is clean; preference is **user-configured** sources.

### Extra consumers

- Rofi / walker example scripts (icon path via `appicon resolve --format png`)
- Notification / window-switcher helper notes — still shell-out only; no SVGL URLs in consumer repos

### CI

- Nightly or `workflow_dispatch` live SVGL smoke (1–2 known titles); not required to merge PRs

## Execution order

1. Clone `bolens/appicon` → `/home/panda/dev/appicon`; scaffold Makefile + docs + LICENSE; first push
2. XDG resolve + fixture tests
3. Cache + SVGL + httptest tests + allowlist
4. PNG output path for GTK/Waybar
5. Full CI workflows + `make check`
6. Cut `v0.1.0` with checksums
7. waybar-config install + dock CSS proof behind settings flag
8. (post-v1) Packaging: Nix flake, Home Manager, AUR; optional release signing
9. (post-v1) CLI polish: completions, man, `--offline`, `cache prune`
10. (post-v1) Pluggable sources: `http-index` + docs for Simple Icons / dashboard-icons `dir` packs; Steam/Snap resolve quality
11. (post-v1) Extra consumers + nightly live SVGL job
