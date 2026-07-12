# appicon

Resolve desktop and brand icons to **local file paths** — for Waybar, Rofi, scripts, and anything else that needs a real icon file.

```bash
appicon resolve firefox
appicon resolve --json --format png --size 24 "VS Code"
appicon resolve --offline some-cached-app
appicon prefetch firefox discord
appicon cache stats
appicon cache prune
```

**Resolve order:** existing path → FreeDesktop icon theme / `.desktop` → configured sources ([SVGL](https://svgl.app/) and/or local packs) → miss.

XDG, SVGL (cache-first), local logo packs (`sources.json`), PNG rasterization, `--offline`, and `cache prune` are implemented. Remaining v1: cut `v0.1.0`, then waybar-config consumer. See [docs/plan.md](docs/plan.md).

**PNG note:** `resolve --format png` prefers `resvg` or `rsvg-convert` on `PATH`, otherwise a pure-Go [oksvg](https://github.com/srwiley/oksvg) fallback. Rasterized files are cached under `$XDG_CACHE_HOME/appicon/raster/`.

**Sources:** optional `$XDG_CONFIG_HOME/appicon/sources.json` — ordered list of `svgl` and `dir` packs (see plan). Default is SVGL only.

## Install (after first release)

```bash
# pinned version + SHA256 — helper will live in waybar-config as install-appicon.sh
curl -fsSL "https://github.com/bolens/appicon/releases/download/vX.Y.Z/appicon_linux_amd64.tar.gz" | tar -xz
install -m 755 appicon ~/.local/bin/appicon
```

Until releases exist:

```bash
git clone https://github.com/bolens/appicon.git
cd appicon
make build
./bin/appicon version
```

## Cache

Remote assets live under `$XDG_CACHE_HOME/appicon` (default `~/.cache/appicon`). XDG hits return theme paths directly and are not copied. Optional overrides: `$XDG_CONFIG_HOME/appicon/overrides.json`.

Brand logos from SVGL are third-party marks — cached for personal use; this project does not redistribute a logo pack.

## Development

```bash
make check-fast   # go test + vet + gofmt
make check        # + golangci-lint + gitleaks + actionlint + markdownlint
make build
```

Agent briefing: [AGENTS.md](AGENTS.md). Contributing: [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT for **code** only. See [LICENSE](LICENSE).
