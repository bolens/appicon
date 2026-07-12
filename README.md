# appicon

Resolve desktop and brand icons to **local file paths** — for Waybar, Rofi, scripts, and anything else that needs a real icon file.

```bash
appicon resolve firefox
appicon resolve --json --format png --size 24 "VS Code"
appicon resolve --offline some-cached-app
appicon prefetch firefox discord
appicon override set my-browser firefox
appicon override list
appicon cache stats
appicon cache prune
appicon mcp   # stdio MCP for agents
appicon daemon            # optional user socket daemon
appicon completion bash   # print completion script
appicon man | man -l -    # view man page
```

**Resolve order:** existing path → FreeDesktop icon theme / `.desktop` → configured sources ([SVGL](https://svgl.app/) and/or local packs) → miss.

XDG, SVGL (cache-first), local logo packs (`sources.json`), PNG rasterization, `--offline`, `cache prune`, MCP, optional socket daemon, and shell completions are implemented. See [docs/plan.md](docs/plan.md).

**Consumer contract:** exit `0` / `1` (miss) / `2` (error); stable `resolve --json` fields — [docs/consumer-contract.md](docs/consumer-contract.md). Misses are supported (callers keep glyphs). Treat appicon like optional peers such as `zscroll` / `cava`: never require the binary for a working bar.

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

## Overrides

Long-tail query remaps live in `$XDG_CONFIG_HOME/appicon/overrides.json`:

```bash
appicon override set my-wm-class firefox
appicon override list --json
```

## MCP (agents)

Run the same binary as a stdio MCP server — tools call `internal/resolve` (no extra download logic):

```bash
appicon mcp
```

| Tool | Mirrors |
|------|---------|
| `resolve` | `appicon resolve --json` |
| `prefetch` | `appicon prefetch` |
| `cache_stats` / `cache_clear` / `cache_prune` | matching `cache` subcommands |
| `override_list` / `override_get` / `override_set` / `override_rm` | `appicon override …` |
| `version` | `appicon version` |

Example Cursor / Claude Desktop snippet:

```json
{
  "mcpServers": {
    "appicon": {
      "command": "appicon",
      "args": ["mcp"]
    }
  }
}
```

Agents should call `resolve` only — never invent SVGL URLs in other repos.

## Daemon (optional)

Long-lived resolve over `$XDG_RUNTIME_DIR/appicon.sock` (mode `0600`). Same allowlists/cache as the CLI. `resolve` dials the socket when present and falls back in-process (`--local` / `APPICON_NO_DAEMON=1` skips dial).

```bash
appicon daemon                          # foreground
# or user systemd — see contrib/systemd/README.md
systemctl --user enable --now appicon.socket
```

Home Manager (Linux): `programs.appicon.daemon.enable = true`.

## Shell completions

```bash
# bash
eval "$(appicon completion bash)"
# or install: appicon completion bash > ~/.local/share/bash-completion/completions/appicon

# zsh
appicon completion zsh > "${fpath[1]}/_appicon"   # then: compinit

# fish
appicon completion fish > ~/.config/fish/completions/appicon.fish
```

## Man page

```bash
appicon man | man -l -
# or: appicon man > /usr/local/share/man/man1/appicon.1
```

## Install

```bash
ver=v0.1.1
arch=$(uname -m)
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
esac
curl -fsSL "https://github.com/bolens/appicon/releases/download/${ver}/appicon_${ver}_linux_${arch}.tar.gz" | tar -xz
install -m 755 appicon ~/.local/bin/appicon
appicon version   # → v0.1.1
```

Checksums: download `SHA256SUMS` (and optionally `SHA256SUMS.sigstore.json`) from the same release.

```bash
# checksums
sha256sum --check --ignore-missing SHA256SUMS

# optional cosign keyless verify (Sigstore)
cosign verify-blob \
  --bundle SHA256SUMS.sigstore.json \
  --certificate-identity-regexp '^https://github.com/bolens/appicon/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  SHA256SUMS
```

Or: `bash scripts/ci/verify-release.sh /path/to/downloaded/assets`.

[waybar-config](https://github.com/bolens/waybar-config) pins this via `make install-appicon`.

### AUR (Arch)

Reference PKGBUILDs live under [packaging/aur/](packaging/aur/):

| Package | Tracks |
|---------|--------|
| `appicon` | Tagged source release |
| `appicon-bin` | Prebuilt release tarball |
| `appicon-git` | Latest git commit on `main` |

Fill checksums (except `-git`) and push to aur.archlinux.org when ready.

### Nix

```bash
nix flake lock    # once
nix run github:bolens/appicon -- version
nix build github:bolens/appicon#appicon-bin   # linux prebuilt (like AUR appicon-bin)
nix build github:bolens/appicon#appicon-git   # source + unstable version (like AUR appicon-git)
# local: see nix/README.md for vendorHash + appicon / appicon-bin / appicon-git
```

Home Manager: `programs.appicon.enable = true` via `homeManagerModules.default` (overlay or set `package` to `appicon` / `appicon-bin` / `appicon-git`). Optional `programs.appicon.daemon.enable = true` for the user socket daemon on Linux.

From source:

```bash
git clone https://github.com/bolens/appicon.git
cd appicon
make build
./bin/appicon version
```

## Cache

Remote assets live under `$XDG_CACHE_HOME/appicon` (default `~/.cache/appicon`). XDG hits return theme paths directly and are not copied.

Optional query remaps: `$XDG_CONFIG_HOME/appicon/overrides.json` — manage with:

```bash
appicon override set steam_app_12345 "Some Game"
appicon override get steam_app_12345
appicon override list --json
appicon override rm steam_app_12345
appicon override path
```

Brand logos from SVGL are third-party marks — cached for personal use; this project does not redistribute a logo pack.

## Examples

Shell-out-only consumers (no SVGL URLs):

```bash
bash examples/rofi-appicon.sh
bash examples/walker-appicon.sh firefox
bash examples/notify-appicon.sh firefox "Hello" "Icon from appicon"
```

## Development

```bash
make check-fast   # go test + vet + gofmt
make check        # + golangci-lint + gitleaks + actionlint + markdownlint
make build
```

Agent briefing: [AGENTS.md](AGENTS.md). Contributing: [CONTRIBUTING.md](CONTRIBUTING.md). Changelog: [CHANGELOG.md](CHANGELOG.md). Consumer contract: [docs/consumer-contract.md](docs/consumer-contract.md).

To cut a release locally (no push): `bash scripts/ci/cut-release.sh v0.1.1`.

## License

MIT for **code** only. See [LICENSE](LICENSE).
