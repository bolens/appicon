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

**Sources:** optional `$XDG_CONFIG_HOME/appicon/sources.json` — ordered `svgl`, local `dir` packs, and `http-index` remotes (explicit host allowlist required). Default is SVGL only.

Example — local [Simple Icons](https://github.com/simple-icons/simple-icons) / [dashboard-icons](https://github.com/homarr-labs/dashboard-icons) clones before SVGL:

```json
{
  "sources": [
    { "type": "dir", "path": "~/.local/share/appicon/packs/dashboard-icons" },
    { "type": "dir", "path": "~/.local/share/appicon/packs/simple-icons/icons" },
    { "type": "svgl" }
  ]
}
```

Do not point `http-index` at third-party CDNs unless you control the allowlist and accept their terms; prefer cloning packs locally.

## Install (after first release)

```bash
ver=v0.1.0
arch=$(uname -m)
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
esac
curl -fsSL "https://github.com/bolens/appicon/releases/download/${ver}/appicon_${ver}_linux_${arch}.tar.gz" | tar -xz
install -m 755 appicon ~/.local/bin/appicon
appicon version   # → v0.1.0
```

Checksums: download `SHA256SUMS` from the same release and verify before install. waybar-config will pin this via `install-appicon.sh`.

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
