# appicon

Resolve desktop and brand icons to **local file paths** — for Waybar, Rofi, scripts, and anything else that needs a real icon file.

```bash
appicon resolve firefox
appicon resolve --json --format png --size 24 "VS Code"
appicon prefetch firefox discord
appicon cache stats
```

**Resolve order:** existing path → FreeDesktop icon theme / `.desktop` → [SVGL](https://svgl.app/) (cached) → miss.

This repository is **scaffolded**; XDG and SVGL backends are stubs. See [docs/plan.md](docs/plan.md) for the full design.

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
