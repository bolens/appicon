# Feature specification: Local icon resolution and optional transports

**Created**: 2026-09-05
**Status**: Retrospective baseline
**Inspected revision**: `60ab2588f13c8c766a30a3b3f5702c142d5a64d7`
**Input**: The owner requested a fleet-wide Spec Kit retrofit and implementation audit.

Appicon resolves desktop and brand queries to local files through ordered local, cached, and optional remote stages. CLI, MCP, and the optional Unix daemon expose the shared resolver.

This specification records existing contracts after implementation. It does not
claim that the original work followed Spec Kit. New behavior requires a separate
change contract. Existing feature specifications remain authoritative within their
own scope.

## User scenarios and testing

### User story 1: Resolve an icon without making the consumer depend on the service (P1)

A bar or launcher asks for a local icon and retains its own fallback on a miss.

**Acceptance**: A fixture hit returns an absolute local path, a miss returns exit 1, and an operational error returns exit 2. JSON output preserves the documented single and batch shapes.

### User story 2: Use cached icons offline (P2)

A caller warms its cache once and resolves without provider requests on subsequent offline calls.

**Acceptance**: A cached fixture resolves with offline mode enabled. An uncached remote-only query produces a supported miss without an HTTP request.

### User story 3: Configure sources and optional transports (P3)

A maintainer configures ordered stages, environment-variable names for provider authentication, local packs, and optional daemon or MCP access.

**Acceptance**: Invalid source configuration is rejected, missing credentials skip their stage with an auth label, and CLI resolution remains available without a daemon.

## Requirements

- **FR-001**: Resolution MUST preserve local-path output, hit/miss/error exit codes, and the documented single and batch JSON schemas.
- **FR-002**: Offline resolution MUST use local sources and cached assets without contacting remote providers.
- **FR-003**: The resolver MUST retain ordered stages and handle supported misses without treating them as operational failure.
- **FR-004**: Configuration MUST store credential environment-variable names rather than secret values, and missing provider credentials MUST skip only that stage.
- **FR-005**: Remote archives, indexes, pack installation, cache deletion, and rasterization MUST retain their allowlists and bounds.
- **FR-006**: MCP and daemon operations MUST use the shared resolver contract. Daemon absence MUST retain in-process operation, and Windows MUST report daemon unavailability.
- **FR-007**: PNG consumer size MUST default to 48 and clamp requests above 512 without changing the JSON result schema.

## Success criteria

- **SC-001**: Every requirement has a named source owner and acceptance check in `coverage.md`.
- **SC-002**: The listed native checks pass for the reviewed candidate, with unavailable environments and operational checks recorded separately.
- **SC-003**: Retrofitting preserves existing interfaces and completed specifications. Any confirmed implementation gap is corrected under an explicit requirement before it is marked complete.

## Edge cases and operational limits

Tests use isolated fixtures and local HTTP servers. No live provider credentials or user caches are acceptance fixtures. Cross-platform compilation and hosted checks do not establish runtime behavior on every operating system. Deferred ideas in docs/deferred.md remain outside this baseline. This documentation change does not require a versioned product release.
