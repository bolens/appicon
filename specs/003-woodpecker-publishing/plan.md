# Implementation plan

Own `.woodpecker.yml`, publication/build/lint helpers, phase-gated Actions,
release documentation and governance references. Reuse native commands and
immutable shared tooling; add bounded mocked trust-boundary tests. Keep build
steps credential-free, constrain the repository-scoped publisher step, and
preserve hosted exceptions. Root owns credentials, host settings, source mirrors
and live cutover. No host APIs or release operations occur in local validation.

Constitution check: preserve domain principles, protected review, offline
fixtures and credential boundaries. Public artifacts contain source identity
and hashes only; operational receipts remain private.
