# Changelog

## [Unreleased]

## [0.2.0] — 2026-07-12

### Added

- CLI↔daemon integration tests; prefetch uses daemon `resolve-batch` when available; `Result.Hint` plumbed end-to-end
- Examples: batch resolve in rofi/walker; `examples/prefetch-and-suggest.sh`
- Docs: CONTRIBUTING checks table, sources MCP/theme, systemd order/explain/batch
- Tests for batch resolve, override suggest, prefetch `--from-desktop`, `__complete`, daemon order/explain/batch, MCP `queries` / `override_suggest` / `from_desktop`, theme/recent/catalog helpers
- Batch JSON schema ([docs/resolve-batch-result.schema.json](docs/resolve-batch-result.schema.json)); CI path filters cover `testdata/**` / `examples/**`; jobs assert `consumer-smoke` + `aur-publish-check`
- Daemon protocol: `order`, `explain`, and `resolve-batch` (CLI no longer forces in-process for `--order`/`--explain`)
- Batch resolve: `appicon resolve --json q1 q2 …` → `{results:[…]}`; MCP `resolve` accepts `queries`
- `appicon override suggest` (+ `--from-misses` / `--apply`); MCP `override_suggest`; recent miss journal under cache
- `appicon prefetch --from-desktop` (+ MCP `from_desktop`) — warm from installed `.desktop` files
- Theme auto-detect from `GTK_THEME` `:dark`/`:light`; XDG prefers `name-dark` / `name-symbolic` / `name-light`
- Shell completions for queries via `appicon __complete queries` (overrides, recent, catalog)
- Consumer smoke: `make check-consumer-smoke`; AUR publish readiness: `make check-aur-publish`
- Repo hygiene: Dependabot, `govulncheck`, `SECURITY.md`, CODEOWNERS, issue/PR templates, `.editorconfig` / `.gitattributes`, pinned `markdownlint-cli`, release build provenance attestations
- Docs hub ([docs/README.md](docs/README.md)) with CI `check-docs-crosslinks` so pages stay crosslinked
- Pin CI Go to 1.25.12 (stdlib GO-2026-5856); cache `Path` no longer creates dirs (Nix `/homeless-shelter` tests)
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
