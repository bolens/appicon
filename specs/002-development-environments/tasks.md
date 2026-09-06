# Tasks

- [x] Add locked shell, source-free image, and engine adapter.
- [x] Pass native and devenv gates, including adapter regressions.
- [ ] Verify Podman and Docker execution; record macOS and Apple evidence.
- [ ] Complete current-head review, CI, protected merge, and cleanup.

The complete Make gate passed inside devenv, and rootless Podman passed all Go package tests plus 10 Python tooling tests. The container includes its CA bundle and C compiler; the shell binds Go 1.25.13 to its matching standard library even when the host exports another GOROOT. Docker and native macOS validation remain pending CI; Apple execution remains unverified.
