# Agent guidance

The project constitution is [.specify/memory/constitution.md](.specify/memory/constitution.md). Read it, then
use [docs/README.md](docs/README.md) to locate the contract relevant to the change.

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

## Planning and evidence

Use the [project guide](.specify/memory/project-guide.md) and
[constitution](.specify/memory/constitution.md) for substantial changes. The guide
owns Spec Kit scope, retained history, retrospective requirements, and acceptance
evidence. Prose maintenance uses the normal repository workflow.

## Context and handoffs

- Search before reading. Use bounded source excerpts for exploratory reads over
  350 lines, and inspect required guidance and actual source before editing.
- When delegation is permitted, assign a bounded question or output, paths, and
  check. Return source locations, changes, and verification gaps for final review.
- Keep durable corrections in the [project guide](.specify/memory/project-guide.md)
  or owning contract. Replace superseded advice and read it before reuse.
  Temporary progress belongs in task notes. Preserve existing authority rules.
