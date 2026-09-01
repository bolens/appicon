# appicon Constitution

## Core Principles

### I. Local-Path Consumer Contract
Resolution MUST return local paths and preserve documented exit codes and JSON schemas. A miss is supported behavior; consumers retain fallbacks and MUST NOT depend on appicon availability.

### II. Cache-First, Offline-Testable Resolution
Warm-cache resolution MUST avoid remote providers. Tests MUST use fixtures or local HTTP servers, and network-backed stages remain ordered, allowlisted, observable, and optional.

### III. Hostile Input and Secret Boundaries
Archives, indexes, paths, provider responses, and install destinations MUST be treated as hostile. Configuration stores secret environment-variable names, never credential values; diagnostics remain redacted.

### IV. One Resolver, Optional Transports
CLI, MCP, and daemon surfaces MUST share resolver behavior. MCP remains thin, the daemon remains optional, and in-process CLI operation remains functional on supported platforms.

### V. Synchronized Contracts and Verification
Behavior changes MUST update their canonical docs, schemas, and tests together. Focused tests precede `make check-fast`; cross-cutting and release work uses the full gate when available.

## Governance

`docs/README.md` maps detailed authorities. Contract or security exceptions require explicit rationale and tests. Amendments use semantic versioning.

**Version**: 1.0.0 | **Ratified**: 2026-08-15 | **Last Amended**: 2026-08-15
