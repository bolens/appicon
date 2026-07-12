# Agent briefing (appicon)

Short rules for AI coding agents in this repository.

## Scope

- This is a standalone Go CLI: `github.com/bolens/appicon`.
- Consumers (e.g. [waybar-config](https://github.com/bolens/waybar-config)) shell out to `appicon resolve` and get a local path.
- Agents can also run `appicon mcp` (stdio MCP) — tools wrap the same resolve path.
- Prefer MCP tools (`resolve`, `sources_*`, `pack_*`, …) over shelling the CLI when MCP is connected.
- Do **not** embed SVGL/CDN URLs or download logic in consumer repos — only call this binary / MCP tools.

## Source of truth

- Deferred ideas (not a backlog): [docs/deferred.md](docs/deferred.md).
- Consumer exit codes / `--json` schema: [docs/consumer-contract.md](docs/consumer-contract.md).
- Resolve stages / packs: [docs/sources.md](docs/sources.md), [docs/packs.md](docs/packs.md).
- Public CLI: `cmd/appicon` (`resolve`, `prefetch`, `status`, `cache`, `override`, `sources`, `pack`, `daemon`, `mcp`, `completion`, `man`, `version`).
- Packages: `internal/resolve`, `internal/xdg`, `internal/svgl`, `internal/pack`, `internal/packs`, `internal/simpleicons`, `internal/dashboardicons`, `internal/githubicon`, `internal/glyph`, `internal/slugcdn`, `internal/httpindex`, `internal/cache`, `internal/raster`, `internal/appmcp`, `internal/completion`, `internal/daemon`.
- Optional daemon: unix socket under `$XDG_RUNTIME_DIR`; never required — CLI falls back in-process.

## Do / don’t

- **Do** keep network cache-first; never hit SVGL on a warm cache.
- **Do** allowlist download hosts (`api.svgl.app`, `svgl.app`).
- **Do** add fixture/`httptest` tests — no live network required to merge.
- **Do** run `make check-fast` before committing.
- **Do** keep MCP tools thin wrappers over `internal/resolve` — no second download path.
- **Do** treat resolve miss (exit `1`) as a supported outcome for consumers.
- **Do** use `appicon override` / MCP `override_*` for long-tail remaps — not speculative aliases in code.
- **Don’t** vendor SVGL’s full catalog into releases.
- **Don’t** commit secrets or live API tokens (none are required today).
- **Don’t** require appicon in consumer bars — optional peer like zscroll/cava (glyph fallback).

## Checks

```bash
make check-fast
make check
```
