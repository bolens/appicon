# Agent guidance

The project constitution is `.specify/memory/constitution.md`. Read it, then
use `docs/README.md` to locate the contract relevant to the change.

- Resolution is cache-first. Warm-cache and test paths must not contact live
  providers; use fixtures or `httptest`.
- Preserve the consumer contract: local paths, exit `1` as a supported miss,
  stable JSON schemas, and caller fallback glyphs. MCP stays a thin wrapper
  over the shared resolver.
- Treat remote indexes, archives, paths, and install destinations as hostile.
  Preserve allowlists, containment, extraction limits, and safe wipe bounds.
- Store only secret environment-variable names in configuration. Never expose
  credential values; missing credentials skip a stage as `stage(auth)`.
- Keep daemon use optional and all CLI operations functional in-process.
- Update the source-of-truth docs and synchronized schemas/contracts together.
- Run focused tests, then `make check-fast`; use `make check` for release-bound
  or cross-cutting work. Report skipped tools instead of claiming they passed.

## Spec-driven changes

Use Spec Kit for new capabilities, architecture, security-sensitive behavior,
migrations, and coordinated multi-file changes. Keep narrow fixes, dependency
updates, prose edits, and release housekeeping in the normal repository
workflow unless their risk warrants a written specification. Keep completed
feature directories under `specs/` as decision history; do not backfill them for
finished work.
