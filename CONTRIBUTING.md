# Contributing

Thanks for improving appicon. Keep changes focused and covered by tests.

## Setup

```bash
git clone https://github.com/bolens/appicon.git
cd appicon
make check-fast
make build
```

## Development loop

1. Implement behind the packages in `internal/`.
2. Add unit tests (XDG fixtures under `testdata/`, SVGL via `httptest`).
3. `make check-fast` locally; `make check` before opening a PR if tools are available.
4. Update [README.md](README.md) / [docs/plan.md](docs/plan.md) when behavior changes.

## Checks

| Target | What |
|--------|------|
| `make check-fast` | `go test`, `go vet`, `gofmt` clean |
| `make check` | check-fast + golangci-lint + gitleaks + actionlint + markdownlint |
| `make build` | `bin/appicon` |

CI runs the same gates on pull requests. Release workflow builds linux amd64/arm64 + `SHA256SUMS` on `v*` tags.

## PRs

- Prefer small PRs.
- Scaffold stubs use `ErrNotImplemented` — replace them rather than layering parallel APIs.
- Do not commit cached logos or secrets.
