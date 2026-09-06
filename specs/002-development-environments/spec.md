# Development environments

Provide a locked devenv shell for Go tests, vet, formatting, and development-helper regressions. Export a source-free Linux tool image usable with Docker, Podman, and Apple container. Keep existing flake packaging, resolver output, optional daemon behavior, provider credentials, and caller caches unchanged.

Acceptance: `devenv test` runs the existing fast gate and Python tooling tests; container commands preserve argument boundaries and caller file ownership. Linux and macOS development use the same locked inputs. Apple execution requires its runtime and a Linux Nix builder; document unavailable platform evidence.
