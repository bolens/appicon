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
2. Add unit tests (XDG fixtures under `testdata/`, SVGL via `httptest`). Cover new CLI/MCP/daemon surfaces with focused tests.
3. `make check-fast` locally; `make check` before opening a PR if tools are available.
4. Update docs via the map in [docs/README.md](docs/README.md): touch the **source of truth** row first, then keep [README.md](README.md), [docs/consumer-contract.md](docs/consumer-contract.md) / [docs/resolve-result.schema.json](docs/resolve-result.schema.json) / [docs/resolve-batch-result.schema.json](docs/resolve-batch-result.schema.json), and stage docs ([docs/sources.md](docs/sources.md) / [docs/packs.md](docs/packs.md)) aligned. Run `make check-docs-crosslinks`.

## Checks

| Target | What |
|--------|------|
| `make check-fast` | `go test`, `go vet`, `gofmt` clean |
| `make check` | check-fast + lint + govulncheck + gitleaks + actionlint + markdownlint + docs crosslinks + consumer smoke + AUR publish readiness + path filters + packaging versions |
| `make build` | `bin/appicon` |
| `make check-docs-crosslinks` | Docs hub links every page; pages link back |
| `make check-consumer-smoke` | Offline resolve JSON (single + batch schemas) + `examples/*.sh` syntax |
| `make check-aur-publish` | Stable/bin PKGBUILDs have real checksums (publish to AUR remains a human step) |
| `make check-ci-path-filters` | Push `on.paths` covers dorny filters; required CI jobs present |

Pull requests require `ci/woodpecker/pr/woodpecker` and the retained
`Development environment` check. Keep every active required check green on the
current head. After publisher cutover, Woodpecker prepares release archives,
the packs bundle and checksums; the retained GitHub bridge signs and attests
with OIDC, and the packaging coordinator merges the verified packaging PR.
Follow the [delivery contract](docs/woodpecker-publishing.md) and
[release playbook](RELEASING.md) for signed tags, publisher selection and rollback.

Markdown lint uses pinned `markdownlint-cli` from `package.json` / `package-lock.json` (`npm ci` locally if you run `make check-markdownlint`).

Docs map: [docs/README.md](docs/README.md). Security: [SECURITY.md](SECURITY.md). Agents: [AGENTS.md](AGENTS.md).

## PRs

- Prefer small PRs.
- Do not invent speculative aliases in code — use `appicon override` / MCP `override_*` (and `override suggest` for candidates).
- Do not commit cached logos or secrets.

## Devcontainer

See [the devcontainer guide](.devcontainer/README.md) for the repository toolchain,
setup, checks and platform limits.

## Woodpecker CI

[The delivery contract](docs/woodpecker-publishing.md) describes native Linux
checks, direct GitHub source identity, retained platform exceptions and the
phased publication cutover. `.woodpecker.yml` also validates reconciled Gitea
copies without publication credentials. Existing Actions remain active until
native exact-head checks and host requirements have migrated. Follow the owning
release playbook; no test creates a public tag, release or AUR push.
