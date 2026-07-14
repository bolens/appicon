# Security Policy

See also: [README.md](README.md) (install / verify), [docs/README.md](docs/README.md) (docs map), [docs/consumer-contract.md](docs/consumer-contract.md) (misses are supported), [docs/sources.md](docs/sources.md) (allowlists / offline).

## Supported versions

Security fixes land on `main` and ship in the next tagged release. Older tags are not backported unless a consumer reports a severe issue.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security-sensitive reports.

- Prefer [GitHub private vulnerability reporting](https://github.com/bolens/appicon/security/advisories/new) on this repository.
- Or email the maintainer via the address on the GitHub profile for [bolens](https://github.com/bolens).

Include: affected version/commit, reproduce steps, impact, and any suggested fix.

## Trust model (what this tool does)

`appicon` resolves icons to **local paths**. Network use is optional and cache-first:

- Warm cache / `--offline` must not hit remotes (including SVGL).
- Downloads are restricted to **allowlisted hosts** (built-in stages such as `api.svgl.app` / `svgl.app`; custom `http-index` only with an explicit host allowlist you control).
- Opt-in CDN / API / GitHub stages are never enabled by default — configure them via `sources.json`/`sources.yaml` / `--order`. Asset downloads for the `github` stage are **HTTPS-only**.
- **Bring-your-own-key (BYOK):** API tokens for Logo.dev, Noun Project, GitHub PAT, and optional `http-index` Bearer auth are read from environment variables named in `token_env` / `secret_env`. Never put secrets in config files. Missing credentials skip that stage.
- The optional daemon uses the same resolve path and allowlists; the socket is under `$XDG_RUNTIME_DIR` with mode `0600`. `appicon status` reports `daemon_alive` via ping.
- **Local packs** (`pack install` / `pack add` / MCP `pack_*`): treat pack trees and archive contents as untrusted input. `index.json` and `--subdir` paths are confined to the pack/install root; archive extracts reject Zip Slip, skip symlink/hardlink members, refuse writes through existing symlinks, cap member/total size, and use fixed file modes. Non-loopback archive URLs require HTTPS; redirects cannot target cloud metadata / link-local hosts. Custom `--path` / MCP `path` may point outside the packs data dir, but `appicon` will not wipe a non-empty tree there (and refuses `/`, `.`, and `$HOME`).
- **Cache / raster:** cache keys must stay under `$XDG_CACHE_HOME/appicon`; PNG raster size is capped at 512px (`resolve` clamps larger `--size` values).

Treat resolve misses (exit `1`) as normal. Do not require `appicon` for a working bar — fall back to glyphs.

## Verifying releases

Release assets include `SHA256SUMS` and a keyless Sigstore bundle `SHA256SUMS.sigstore.json`. See [README.md](README.md) for `sha256sum` / `cosign verify-blob` examples.

GitHub also publishes build provenance attestations for release tarballs (and related artifacts). Verify with:

```bash
gh attestation verify dist/appicon_vX.Y.Z_linux_amd64.tar.gz \
  --repo bolens/appicon
```

## Scope notes

- Do not commit secrets or live API tokens. Use `token_env` / `secret_env` pointing at your own env / secret manager (sops, agenix, etc.).
- Do not vendor third-party icon catalogs into this repository’s releases unless via the optional packs bundle workflow, which carries upstream licenses in-tree.
