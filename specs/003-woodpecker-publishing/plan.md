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

The native publisher hands an exact immutable packaging intent to a retained
forge-only GitHub coordinator and exits pending. Waiting on PR checks inside
both single-capacity CI agents would starve those checks. The coordinator alone
waits and uses normal protected exact-head squash; it performs no build, test
suite or release publication. A verified asynchronous receipt closes delivery.
Ordinary PR metadata checks use webhook/Git and public release redirects to
avoid consuming shared unauthenticated REST quotas; no PR credential is added.
