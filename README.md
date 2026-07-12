# appicon

Resolve desktop and brand icons to **local file paths** — for Waybar, Rofi, scripts, and anything else that needs a real icon file.

```bash
appicon resolve firefox
appicon resolve --json --format png --size 24 "VS Code"
appicon resolve --json firefox discord
appicon resolve --offline some-cached-app
appicon resolve --explain missing-app
appicon prefetch firefox discord
appicon prefetch --from-desktop
appicon prefetch --json --offline firefox
appicon override set my-browser firefox
appicon override suggest my-browser
appicon override list
appicon sources get --json
appicon sources set --file ./sources.json
appicon status
appicon cache stats
appicon cache prune
appicon mcp   # stdio MCP for agents
appicon daemon            # optional user socket daemon
appicon completion bash   # print completion script
appicon man | man -l -    # view man page
```

**Resolve order (default):** file → overrides → XDG / `.desktop` → [SVGL](https://svgl.app/). Fully reorderable via `sources.json` / `sources.yaml` / `--order` — including opt-in `simple-icons`, `dashboard-icons`, `github`, BYOK (`logo-dev`, `iconify`, `noun-project`), `glyph`, and local packs. See [docs/sources.md](docs/sources.md) and [docs/packs.md](docs/packs.md).

XDG, SVGL (cache-first), local packs, opt-in CDN/github/BYOK/glyph stages, PNG rasterization, `--offline`, `cache prune`, MCP, optional unix-socket daemon (not on Windows), and shell completions are implemented. Deferred ideas: [docs/deferred.md](docs/deferred.md).

**Consumer contract:** exit `0` / `1` (miss) / `2` (error); stable `resolve --json` fields (single object or `{results:[…]}` batch) — [docs/consumer-contract.md](docs/consumer-contract.md), schemas [docs/resolve-result.schema.json](docs/resolve-result.schema.json) / [docs/resolve-batch-result.schema.json](docs/resolve-batch-result.schema.json). Misses are supported (callers keep glyphs). Auth-skipped BYOK stages show as `stage(auth)` in `--explain` `tried`. Treat appicon like optional peers such as `zscroll` / `cava`: never require the binary for a working bar.

**Portability:** Primary target is Linux (XDG, Flatpak/Snap roots, systemd daemon). macOS/Windows build and run in-process resolve; config/cache fall back to OS user dirs when `XDG_*` are unset. Daemon refuses on Windows (`daemon_supported=false` in `status`).

**PNG note:** `resolve --format png` prefers `resvg` or `rsvg-convert` on `PATH`, otherwise a pure-Go [oksvg](https://github.com/srwiley/oksvg) fallback. Rasterized files are cached under `$XDG_CACHE_HOME/appicon/raster/`.

**Theme note:** `--theme dark|light`, `APPICON_THEME`, or `GTK_THEME` suffix (`Adwaita:dark`) prefer matching SVGL/CDN and XDG variants (`name-dark` / `name-symbolic` / `name-light`). Icon **theme name** is separate (`APPICON_ICON_THEME`).

**Sources:** `$XDG_CONFIG_HOME/appicon/sources.json` (or `.yaml`) — every stage is an ordered entry. Default without a file is `file → overrides → xdg → svgl`. Opt-in remotes are never enabled by default. BYOK stages take `token_env` / `secret_env` (env var *names* whose values hold secrets — never put keys or secret paths in config).

```bash
appicon sources list
appicon sources get --json
appicon pack install simple-icons   # local clone + register
appicon pack install --name mine --subdir icons https://github.com/org/my-icons.git
appicon resolve --order glyph,svgl,xdg my-app
appicon resolve --order logo-dev,xdg shopify.com   # needs LOGO_DEV_TOKEN
appicon status
```

Example — remaps and a personal pack before path/XDG/SVGL:

```json
{
  "sources": [
    { "type": "overrides" },
    { "type": "pack", "name": "mine", "path": "~/.local/share/appicon/packs/mine" },
    { "type": "file" },
    { "type": "xdg" },
    { "type": "svgl" },
    { "type": "simple-icons" }
  ]
}
```

CDN stages (`simple-icons` / `dashboard-icons`) are separate from **local** `pack install` clones of the same upstreams. Do not point `http-index` at third-party CDNs unless you control the allowlist and accept their terms.

## Overrides

Long-tail query remaps live in `$XDG_CONFIG_HOME/appicon/overrides.json` (or `.yaml`):

```bash
appicon override set my-wm-class firefox
appicon override list --json
appicon override export --format yaml > overrides.yaml
appicon override import --merge --file overrides.yaml
```

## MCP (agents)

Run the same binary as a stdio MCP server — tools call `internal/resolve` (no extra download logic):

```bash
appicon mcp
```

| Tool | Mirrors |
|------|---------|
| `resolve` | `appicon resolve --json` (optional `order`, `explain`, `queries` batch; miss → `path:null`, not IsError) |
| `prefetch` | `appicon prefetch` (optional `order`, `offline`, `theme`, `from_desktop`, `json`) |
| `status` | `appicon status --json` |
| `sources_list` / `sources_get` / `sources_set` | `appicon sources list\|get\|set` |
| `pack_list` / `pack_path` / `pack_add` / `pack_install` / `pack_update` / `pack_install_bundle` | `appicon pack …` (`pack_install`: `recipe` or `url`, plus `name`/`subdir`/`ref`) |
| `cache_stats` / `cache_clear` / `cache_prune` | matching `cache` subcommands |
| `override_list` / `override_get` / `override_set` / `override_rm` / `override_suggest` / `override_export` / `override_import` | `appicon override …` |
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

Agents should prefer MCP tools over shelling `appicon` when MCP is connected. Call `resolve` / `sources_*` / `pack_*` only — never invent CDN or SVGL URLs in other repos.

## Daemon (optional)

Long-lived resolve over `$XDG_RUNTIME_DIR/appicon.sock` (mode `0600`). Same allowlists/cache as the CLI, including order, explain, and batch. `resolve` / `prefetch` dial the socket when present and fall back in-process (`--local` / `APPICON_NO_DAEMON=1` skips dial).

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
ver=v0.2.1
arch=$(uname -m)
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
esac
curl -fsSL "https://github.com/bolens/appicon/releases/download/${ver}/appicon_${ver}_linux_${arch}.tar.gz" | tar -xz
install -m 755 appicon ~/.local/bin/appicon
appicon version   # → v0.2.1
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

# optional GitHub build provenance attestation
gh attestation verify "appicon_${ver}_linux_${arch}.tar.gz" --repo bolens/appicon
```

Or: `bash scripts/ci/verify-release.sh /path/to/downloaded/assets`.

See [SECURITY.md](SECURITY.md) for reporting vulnerabilities and the trust model.

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
appicon override export --format yaml
appicon override import --merge --file overrides.yaml
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
make check        # + golangci-lint + govulncheck + gitleaks + actionlint + markdownlint + docs crosslinks
make build
```

## Documentation

Canonical map (update the listed source of truth when behavior changes): **[docs/README.md](docs/README.md)**.

| Topic | Doc |
|-------|-----|
| Exit codes / `resolve --json` | [docs/consumer-contract.md](docs/consumer-contract.md), [docs/resolve-result.schema.json](docs/resolve-result.schema.json) |
| Stages / `sources.json` | [docs/sources.md](docs/sources.md) |
| Local packs / recipes | [docs/packs.md](docs/packs.md) |
| Not a backlog | [docs/deferred.md](docs/deferred.md) |
| Security / verify releases | [SECURITY.md](SECURITY.md) |
| Agents | [AGENTS.md](AGENTS.md) |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) |
| Nix / AUR / systemd | [nix/README.md](nix/README.md), [packaging/aur/README.md](packaging/aur/README.md), [contrib/systemd/README.md](contrib/systemd/README.md) |

To cut a release locally (no push): `bash scripts/ci/cut-release.sh v0.2.1`.

## License

MIT for **code** only. See [LICENSE](LICENSE).
