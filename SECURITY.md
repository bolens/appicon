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
- Opt-in CDN / GitHub stages are never enabled by default — configure them via `sources.json` / `--order`.
- The optional daemon uses the same resolve path and allowlists; the socket is under `$XDG_RUNTIME_DIR` with mode `0600`.

Treat resolve misses (exit `1`) as normal. Do not require `appicon` for a working bar — fall back to glyphs.

## Verifying releases

Release assets include `SHA256SUMS` and a keyless Sigstore bundle `SHA256SUMS.sigstore.json`. See [README.md](README.md) for `sha256sum` / `cosign verify-blob` examples.

GitHub also publishes build provenance attestations for release tarballs (and related artifacts). Verify with:

```bash
gh attestation verify dist/appicon_vX.Y.Z_linux_amd64.tar.gz \
  --repo bolens/appicon
```

## Scope notes

- Do not commit secrets or live API tokens (none are required today).
- Do not vendor third-party icon catalogs into this repository’s releases unless via the optional packs bundle workflow, which carries upstream licenses in-tree.
