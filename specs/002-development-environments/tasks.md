# Tasks

- [x] Add locked shell, source-free image, and engine adapter.
- [x] Pass native and devenv gates, including adapter regressions.
- [x] Verify native Linux/macOS and Linux Docker checks on the recorded main revision.
- [x] Verify merged source delivery and the applicable main-revision workflows.

Historical pre-merge observation (superseded by the receipt below):
The complete Make gate passed inside devenv, and rootless Podman passed all Go package tests plus 10 Python tooling tests. The container includes its CA bundle and C compiler; the shell binds Go 1.25.13 to its matching standard library even when the host exports another GOROOT. Docker and native macOS validation remain pending CI; Apple execution remains unverified.

## Delivery verification — 2026-09-06

The [development workflow](https://github.com/bolens/appicon/actions/runs/34028950335) passed on
`9633dd6f119ab28d0864dd6dd3ed304adfd27d18`. Both native platform jobs ran successfully;
the Linux job also executed and passed the Docker development-image check. All
applicable workflows observed for that main revision completed successfully.

Actual Apple container-engine execution remains unverified. Native macOS devenv
validation does not establish that engine's runtime behavior. Existing live-host
and optional dependency limits still apply. Checkout cleanup remains part of each
task's delivery procedure and is not inferred from CI success.
