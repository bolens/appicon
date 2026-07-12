# Agent briefing (appicon)

Short rules for AI coding agents in this repository.

## Scope

- This is a standalone Go CLI: `github.com/bolens/appicon`.
- Consumers (e.g. [waybar-config](https://github.com/bolens/waybar-config)) shell out to `appicon resolve` and get a local path.
- Agents can also run `appicon mcp` (stdio MCP) — tools wrap the same resolve path.
- Do **not** embed SVGL URLs or download logic in consumer repos — only call this binary / MCP tools.

## Source of truth

- Design / remaining work: [docs/plan.md](docs/plan.md) (also mirrored under `.cursor/plans/`).
- Public CLI: `cmd/appicon` (`resolve`, `prefetch`, `cache`, `mcp`, `completion`, `man`, `version`).
- Packages: `internal/resolve`, `internal/xdg`, `internal/svgl`, `internal/pack`, `internal/httpindex`, `internal/cache`, `internal/raster`, `internal/appmcp`, `internal/completion`.

## Do / don’t

- **Do** keep network cache-first; never hit SVGL on a warm cache.
- **Do** allowlist download hosts (`api.svgl.app`, `svgl.app`).
- **Do** add fixture/`httptest` tests — no live network required to merge.
- **Do** run `make check-fast` before committing.
- **Do** keep MCP tools thin wrappers over `internal/resolve` — no second download path.
- **Don’t** vendor SVGL’s full catalog into releases.
- **Don’t** commit secrets or live API tokens (none are required today).

## Checks

```bash
make check-fast
make check
```
