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
4. Update docs via the map in [docs/README.md](docs/README.md): touch the **source of truth** row first, then keep [README.md](README.md), [docs/consumer-contract.md](docs/consumer-contract.md) / [docs/resolve-result.schema.json](docs/resolve-result.schema.json), and stage docs ([docs/sources.md](docs/sources.md) / [docs/packs.md](docs/packs.md)) aligned. Run `make check-docs-crosslinks`.

## Checks

| Target | What |
|--------|------|
| `make check-fast` | `go test`, `go vet`, `gofmt` clean |
| `make check` | check-fast + golangci-lint + govulncheck + gitleaks + actionlint + markdownlint + docs crosslinks |
| `make build` | `bin/appicon` |
| `make check-docs-crosslinks` | Docs hub links every page; pages link back |

CI runs the same gates on pull requests (required check: **CI result**). Release workflow builds linux amd64/arm64 + `SHA256SUMS`, keyless cosign, and build provenance attestations on `v*` tags.

Markdown lint uses pinned `markdownlint-cli` from `package.json` / `package-lock.json` (`npm ci` locally if you run `make check-markdownlint`).

Docs map: [docs/README.md](docs/README.md). Security: [SECURITY.md](SECURITY.md). Agents: [AGENTS.md](AGENTS.md).

## PRs

- Prefer small PRs.
- Scaffold stubs use `ErrNotImplemented` — replace them rather than layering parallel APIs.
- Do not commit cached logos or secrets.
