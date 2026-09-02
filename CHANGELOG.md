# Changelog

## [Unreleased]

### Added

- Pages code examples use theme-aware shell syntax highlighting without changing copied commands.

## [0.4.1] - 2026-08-31

### Changed

- Updated `golang.org/x/sys` to 0.47.0

### Security

- Release and CI builds use Go 1.25.13 to include standard-library vulnerability fixes
- Refreshed the Nix Go module hash for the current dependency graph

## [0.4.0] - 2026-08-08

### Added

- Portable release archives for macOS arm64 and Windows amd64, with cross-platform checksum generation and release verification
- Release contract tests that validate archive contents, version metadata, complete asset sets, and corrupt-asset rejection
- Nested Linux desktop-entry discovery and hidden-entry handling

### Changed

- Resolution remains cache-first across GitHub, Noun Project, and catalog-backed sources, including global offline mode and canceled requests
- Cache, catalog, configuration, and pack operations use stronger locking, atomic replacement, path containment, symlink checks, and rollback behavior across platforms
- Pack and bundle installs preflight configuration and hostile archive input, cap entries and expansion, validate git refs and subdirectories, and protect pack roots and destinations
- Remote providers enforce redirect allowlists and validate cached/downloaded artifacts, response identity, credential environment names, and configurable hosts
- Native app discovery uses platform-specific paths and scoped fingerprints, with portable XDG fallbacks and cache invalidation for nested applications
- Daemon batch requests and deterministic pack lookups are bounded; daemon responses validate request identity and preserve miss hints

### Fixed

- Wrapped Noun Project downloads, current catalog decoding, URL extension sanitization, and repointed SVGL asset invalidation
- Windows cache locking and file replacement, macOS checksum portability, and release asset completeness checks

## [0.3.0] - 2026-08-08

### Added

- Installed-app icon discovery for macOS XML `Info.plist` app bundles and Windows `.url` shortcuts
- Portable home-relative path expansion, Windows icon search roots, file extensions, and HiDPI icon variants
- Coverage for CDN/glyph fallbacks, MCP cache and override operations, daemon framing, platform paths, native metadata, and download limits

### Changed

- Remote caches now include complete provider identity, URL, repository path/ref, and configurable endpoint data to prevent cross-source collisions
- Remote response bodies are bounded before caching; cache locks, recent-miss writes, configuration writes, overrides, and raster output use safer serialized or atomic replacement
- CLI JSON and plain output paths now propagate write failures
- Daemon framing handles partial writes and listener cleanup preserves active replacement sockets
- Pack queries reject empty normalized names
- CI covers the bounded-read package and fails closed when checking Nix secret configuration

## [0.2.2] - 2026-07-14

### Security

- Pack installs contain subdirectories and destinations, reject unsafe targets and links, require HTTPS for remote archives, cap extraction size, and reject option-like Git remotes
- Cache path containment on `Path` / `WriteAtomic` / `Read` / `Exists`
- GitHub icon downloads: HTTPS only
- Raster: max edge 512px (`raster.MaxSize`); `resolve` clamps `--size` / MCP `size` to that max

## [0.2.1] - 2026-07-12

### Added

- BYOK stages: `logo-dev`, `iconify`, `noun-project`; GitHub PAT + private repo Contents API; optional Bearer on `http-index` via `token_env`
- Sources/overrides as JSON **or** YAML (`sources.yaml` / `overrides.yaml`); `sources set --format`; [docs/sources.schema.json](docs/sources.schema.json)
- `appicon status` / MCP `status`: `daemon_alive`, `credentials`, `goos`/`goarch`, `daemon_supported`
- `appicon override export|import` (+ MCP `override_export` / `override_import`)
- Home Manager: declarative `sources` / `overrides` / `environment` / `environmentFiles` / `configFormat`
- Auth-skip visibility: explain `tried` labels `stage(auth)` when BYOK env is missing
- CI: unit-test matrix covers `logodev` / `iconify` / `nounproject` / `version`; `windows-compile` job; matrix completeness check
- Daemon: unix-only `CloseOnExec` helper so `GOOS=windows` builds cleanly

### Changed

- Portability: Linux-only Flatpak/Snap/`/usr` XDG defaults; Windows config/cache/packs via OS user dirs; safer daemon runtime fallback; `daemon` refused on Windows
- Home Manager: document sops EnvironmentFile (secret *values*); do not put `sops.secrets.*.path` into `environment`

## [0.2.0] - 2026-07-12

### Added

- Batch resolution, desktop-entry prefetch, configurable source ordering, icon packs, and override suggestions.
- New CLI and MCP commands for sources, packs, overrides, recent misses, and batch queries.
- Opt-in Simple Icons, Dashboard Icons, GitHub, and glyph providers.
- Consumer examples, schemas, completions, release attestations, and packaging readiness checks.

### Changed

- The daemon supports ordered, explained, and batched resolution without forcing in-process fallback.
- Theme lookup recognizes dark and light variants and installed desktop entries.
- CI checks consumer behavior, AUR publication, documentation links, vulnerabilities, and Nix builds.

### Fixed

- Cache paths no longer create directories during read-only lookups.
- Miss tracking, daemon framing, and batch results preserve caller-visible hints and errors.

## [0.1.2] - 2026-07-12

### Added

- Home Manager `programs.appicon.daemon.enable` (user systemd socket)
- Nix packages install `lib/systemd/user/` units with absolute `ExecStart`
- `appicon override list|get|set|rm|path` (+ MCP `override_*` tools)
- `flake.lock` for reproducible Nix inputs
- CI packaging gates: version sync + AUR/Nix build matrix for `appicon` / `appicon-bin` / `appicon-git`
- Release workflow waits for a successful CI run on the tagged commit before publishing

### Changed

- AUR PKGBUILDs pin systemd `ExecStart` to `/usr/bin/appicon daemon`
- GitHub Actions: checkout v7, setup-go v6, golangci-lint-action v9, cosign-installer v4, action-gh-release v3, nix-installer v22

## [0.1.1] - 2026-07-12

Post-v0.1.0 packaging and agent/daemon surface. Cut after pushing `main` and tagging `v0.1.1`.

### Added

- Stdio MCP server: `appicon mcp` (`resolve`, `prefetch`, `cache_*`, `version`)
- Shell completions: `appicon completion bash|zsh|fish`
- Man page: `appicon man`
- Optional unix-socket daemon: `appicon daemon` + `contrib/systemd/`
- Nix flake + Home Manager module (`flake.nix`, `nix/home-manager.nix`)
- Nightly live SVGL smoke workflow
- Consumer examples: `examples/{rofi,walker,notify}-appicon.sh`
- AUR reference PKGBUILDs: `appicon`, `appicon-bin`, `appicon-git`
- Cosign keyless signing of release `SHA256SUMS` → `SHA256SUMS.sigstore.json`
- `scripts/ci/verify-release.sh`

### Changed

- CI/release Go toolchain pin: 1.25.x
- Release tarballs include completions, man page, and systemd units

## [0.1.0] - 2026-07-12

Initial release: XDG/SVGL/packs resolve, PNG raster, offline/prune, Waybar consumer.
