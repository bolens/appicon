# Requirement coverage

| Requirement | Source and acceptance evidence |
| --- | --- |
| FR-001 | `cmd/appicon`, `internal/resolve/batch.go`, `docs/consumer-contract.md`, both result schemas, and `scripts/ci/consumer-smoke.sh`. |
| FR-002 | `internal/resolve/resolve.go` forwards offline policy to stages. `resolve_test.go` covers environment offline mode, cached SVGL hits, and remote misses; `stages_extra_test.go` covers CDN misses. |
| FR-003 | `internal/resolve/resolve.go:resolvePipeline`, source configuration tests, and `features_more_test.go` batch/cancellation cases. |
| FR-004 | `internal/resolve/config_auth_test.go`, `byok_pipeline_test.go`, and `docs/sources.md`. |
| FR-005 | `internal/packs`, `internal/httpindex`, `internal/cache`, `internal/raster`, their package tests, and `SECURITY.md`. |
| FR-006 | `internal/appmcp/server.go`, `internal/daemon`, CLI routing, MCP miss/batch tests, and daemon platform tests. |
| FR-007 | `internal/resolve/resolve.go:resolvePipeline`, `internal/raster`, and the consumer contract size tests. |

## Verification receipt

On 2026-09-05, `make check-fast`, `make check-docs-crosslinks`, and `make check-consumer-smoke` passed in the isolated candidate checkout. The checks used fixture data; no live provider credentials were supplied. Separate review and hosted delivery remain pending. A separate self-review traced resolver stages, offline/auth outcomes, raster size bounds, daemon fallback, and consumer-schema tests. The full delivery gate is being checked in a standalone clone because the pinned Go toolchain ignores linked-worktree .git files during VCS discovery.
