# Agent briefing (appicon)

Short rules for AI coding agents in this repository.

## Scope

- This is a standalone Go CLI: `github.com/bolens/appicon`.
- Consumers (e.g. [waybar-config](https://github.com/bolens/waybar-config)) shell out to `appicon resolve` and get a local path.
- Agents can also run `appicon mcp` (stdio MCP) — tools wrap the same resolve path.
- Prefer MCP tools (`resolve`, `sources_*`, `pack_*`, `override_*` including `override_export` / `override_import`, …) over shelling the CLI when MCP is connected.
- Do **not** embed SVGL/CDN URLs or download logic in consumer repos — only call this binary / MCP tools.
- BYOK providers (`logo-dev`, `iconify`, `noun-project`, `github` PAT, `http-index` Bearer): configure `token_env` / `secret_env` in sources config; never commit API keys. Missing env → stage skipped as `stage(auth)` in explain `tried`; check `status.credentials`.

## Source of truth

Documentation map (start here when adding or renaming docs): [docs/README.md](docs/README.md).

- Deferred ideas (not a backlog): [docs/deferred.md](docs/deferred.md).
- Consumer exit codes / `--json` schema: [docs/consumer-contract.md](docs/consumer-contract.md), [docs/resolve-result.schema.json](docs/resolve-result.schema.json), [docs/resolve-batch-result.schema.json](docs/resolve-batch-result.schema.json).
- Resolve stages / packs: [docs/sources.md](docs/sources.md), [docs/packs.md](docs/packs.md).
- Public CLI: `cmd/appicon` (`resolve`, `prefetch`, `status`, `cache`, `override`, `sources`, `pack`, `daemon`, `mcp`, `completion`, `man`, `version`).
- Packages: `internal/resolve`, `internal/xdg`, `internal/svgl`, `internal/pack`, `internal/packs`, `internal/simpleicons`, `internal/dashboardicons`, `internal/githubicon`, `internal/logodev`, `internal/iconify`, `internal/nounproject`, `internal/glyph`, `internal/slugcdn`, `internal/httpindex`, `internal/cache`, `internal/raster`, `internal/appmcp`, `internal/completion`, `internal/daemon`.
- Optional daemon: unix socket under `$XDG_RUNTIME_DIR`; never required — CLI falls back in-process. Not supported on Windows (`daemon_supported=false`).
- Bulk remaps: `appicon override export|import` / MCP `override_export` / `override_import`.

## Do / don’t

- **Do** keep network cache-first; never hit SVGL on a warm cache.
- **Do** allowlist download hosts (`api.svgl.app`, `svgl.app`, plus documented opt-in stage hosts).
- **Do** add fixture/`httptest` tests — no live network required to merge.
- **Do** run `make check-fast` before committing.
- **Do** keep MCP tools thin wrappers over `internal/resolve` — no second download path.
- **Do** treat resolve miss (exit `1`) as a supported outcome for consumers.
- **Do** use `appicon override` / MCP `override_*` for long-tail remaps — not speculative aliases in code.
- **Do** use `token_env` / `secret_env` (env names only) for BYOK APIs — never inline secrets in sources/overrides.
- **Do** treat pack archives / `index.json` / install destinations as untrusted; rely on pack path containment (never assume wipe/`http` archive installs are unrestricted).
- **Don’t** vendor SVGL’s full catalog into releases.
- **Don’t** commit secrets or live API tokens.
- **Don’t** require appicon in consumer bars — optional peer like zscroll/cava (glyph fallback).

## Checks

```bash
make check-fast
make check
```

Security / reporting: [SECURITY.md](SECURITY.md). Docs hub: [docs/README.md](docs/README.md). Contributing: [CONTRIBUTING.md](CONTRIBUTING.md).
