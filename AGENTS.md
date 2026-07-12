# Agent briefing (appicon)

Short rules for AI coding agents in this repository.

## Scope

- This is a standalone Go CLI: `github.com/bolens/appicon`.
- Consumers (e.g. [waybar-config](https://github.com/bolens/waybar-config)) shell out to `appicon resolve` and get a local path.
- Do **not** embed SVGL URLs or download logic in consumer repos — only call this binary.

## Source of truth

- Design / remaining work: [docs/plan.md](docs/plan.md) (also mirrored under `.cursor/plans/`).
- Public CLI: `cmd/appicon`.
- Packages: `internal/resolve`, `internal/xdg`, `internal/svgl`, `internal/cache`, `internal/raster`.

## Do / don’t

- **Do** keep network cache-first; never hit SVGL on a warm cache.
- **Do** allowlist download hosts (`api.svgl.app`, `svgl.app`).
- **Do** add fixture/`httptest` tests — no live network required to merge.
- **Do** run `make check-fast` before committing.
- **Don’t** vendor SVGL’s full catalog into releases.
- **Don’t** commit secrets or live API tokens (none are required today).

## Checks

```bash
make check-fast
make check
```
