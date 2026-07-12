# Changelog

## [Unreleased]

### Added

- Fully reorderable resolve stages (`file`, `overrides`, `xdg`, packs, SVGL, opt-in CDN/github/glyph) via `sources.json` and `resolve --order`
- `appicon sources` / `appicon pack` (+ MCP `sources_*` / `pack_*`); pack recipes, URL install (git / `.tar.gz`), update, and `--from-bundle`
- Opt-in `simple-icons`, `dashboard-icons`, `github`, and `glyph` stages
- Docs: [docs/sources.md](docs/sources.md), [docs/packs.md](docs/packs.md); deferred ideas in [docs/deferred.md](docs/deferred.md)

## [0.1.2] — 2026-07-12

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

## [0.1.1] — 2026-07-12

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

## [0.1.0] — 2026-07-12

Initial release: XDG/SVGL/packs resolve, PNG raster, offline/prune, Waybar consumer.
