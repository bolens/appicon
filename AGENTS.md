# Agent notes

- Keep resolution cache-first. A warm cache must not contact a remote provider,
  and tests must use fixtures or `httptest`, never live services.
- Consumers receive local paths from the CLI or MCP server. Do not move provider
  URLs or download logic into consumer repositories, and keep MCP as a thin
  wrapper over the shared resolver.
- A resolve miss (exit 1) is supported behavior. Consumers must retain their
  fallback glyph and must not require appicon to start.
- Remote hosts stay allowlisted. BYOK configuration stores only environment
  variable names (`token_env`/`secret_env`), never credentials; missing
  credentials skip the stage and appear as `stage(auth)` in explanations.
- Treat pack archives, indexes, subdirectories, and install destinations as
  hostile input. Preserve path containment, extraction limits, and safe wipe
  boundaries.
- Prefer user overrides for long-tail remaps instead of speculative built-in
  aliases. Do not vendor the full SVGL catalog.
- The daemon is optional and unsupported on Windows; all CLI operations must
  continue to work in-process.
- Start documentation changes at [docs/README.md](docs/README.md), and keep the
  linked contract/schema pages synchronized.
- Test new behavior and edge cases, then run `make check-fast`; run `make check`
  before handing off or committing release-bound work.
