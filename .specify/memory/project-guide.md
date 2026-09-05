# appicon Spec Kit project guide

A cache-first Go icon resolver with CLI, optional daemon, and thin MCP surfaces.

Read this guide with `AGENTS.md` and `.specify/memory/constitution.md` before
specifying, planning, or implementing a substantial change. It is project-owned
guidance, not an upstream-managed template.

## Source and ownership map

- `cmd/`
- `internal/`
- `docs/README.md`
- `testdata/`
- `examples/`

## Specification and plan decisions

Specify local-path output, JSON compatibility, exit behavior, resolution stage order,
and consumer fallback. Place shared behavior in the resolver and keep transport wrappers
thin. Identify index, archive, extraction, and cache-containment boundaries for remote
inputs.

## Acceptance evidence

Cover warm-cache resolution with no provider requests, a supported miss, operational
failure, malformed provider input, and in-process behavior without a daemon. Use
fixtures or local HTTP servers and isolated caches.

## Validation and operational limits

```sh
make check-fast
make check
```

Select focused Go tests from the touched package before wider gates. Do not use live
provider credentials or caller caches as test fixtures. Report unavailable platform
checks separately from passing tests.

## Working through Spec Kit

Use Spec Kit for new capabilities, architectural or security-sensitive changes,
migrations, and coordinated changes that need a written contract. Keep narrow fixes,
dependency updates, and prose maintenance in the normal PR workflow.

For a new feature, record observable acceptance criteria in `spec.md`, source ownership
and constitution checks in `plan.md`, and evidence-bearing work in `tasks.md` under the
feature directory created by Spec Kit. Resolve material unknowns before implementation.
Mark tasks complete only after their stated verification, and distinguish completed,
skipped, blocked, and manual checks. Retain completed feature documents as decision
history; do not backfill feature specifications for already finished code.

Keep `.specify/templates/`, `.specify/scripts/`, and generated Codex skills under their
integration manifests. Use this guide and the constitution for local customization.
Regenerate managed files through Spec Kit and verify that project-owned memory survives
updates. Follow `RELEASING.md` for push, merge, release or delivery, and recovery.
