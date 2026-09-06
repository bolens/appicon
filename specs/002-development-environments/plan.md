# Implementation plan

Own root devenv files, ignored state, the development container helper and its process-contract tests, environment CI, and linked development documentation. Use Go 1.25 with common GNU utilities and Python. Preserve `flake.nix`, `flake.lock`, and `nix/` as the package and Home Manager contract.

Validate existing fast and full Make gates, pinned environment evaluation, actual Podman execution, and Linux Docker/native macOS CI. Tests use local fixtures; no live provider tokens, daemon installation, or caller caches. Follow RELEASING.md for protected delivery; development tooling alone does not change the appicon release version.
