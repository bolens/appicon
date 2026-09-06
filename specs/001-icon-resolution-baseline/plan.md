# Plan: Local icon resolution and optional transports

The [specification](spec.md) preserves existing behavior. Use the project guide
and constitution for implementation constraints. Keep upstream-managed templates,
helpers, and integration manifests unchanged.

## Source ownership

- `cmd/appicon`
- `internal/resolve`
- `internal/cache`
- `internal/packs`
- `internal/appmcp`
- `internal/daemon`
- `internal/raster`
- `docs/consumer-contract.md`
- `docs/sources.md`
- `docs/packs.md`

## Constitution check

The baseline preserves cache-first behavior, optional consumers/transports, hostile-input defenses, and synchronized public contracts. It adds no provider, credential, network policy, install destination, or release behavior.

## Validation

```sh
make check-fast
make check-docs-crosslinks
make check-consumer-smoke
```

Run checks in an isolated checkout. Commands are instructions, not evidence of
a pass. Record results in `coverage.md`, keep incomplete work in `tasks.md`, and
follow `RELEASING.md` for reviewed delivery. No live operation is required solely
to create this retrospective baseline.
