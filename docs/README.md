# Documentation

Hub for appicon docs. Prefer editing the **source of truth** column when behavior changes; keep sibling pages linked so nothing orphans.

| Doc | Source of truth for |
|-----|---------------------|
| [../README.md](../README.md) | Install, quickstart, MCP/daemon overview |
| [consumer-contract.md](consumer-contract.md) | Exit codes, `resolve --json` fields (single + batch), consumer rules |
| [resolve-result.schema.json](resolve-result.schema.json) | Machine-readable JSON schema for a single resolve result |
| [resolve-batch-result.schema.json](resolve-batch-result.schema.json) | Batch `{results:[…]}` envelope schema |
| [sources.md](sources.md) | `sources.json`/`sources.yaml` stages, BYOK `token_env`, `--order`, offline/MCP notes |
| [sources.schema.json](sources.schema.json) | Machine-readable schema for sources config |
| [packs.md](packs.md) | Local packs, recipes, CDN vs pack, bundle artifact |
| [deferred.md](deferred.md) | Explicitly **not** a backlog |
| [../SECURITY.md](../SECURITY.md) | Vulnerability reporting, trust model, release verify |
| [../AGENTS.md](../AGENTS.md) | Rules for coding agents / MCP usage |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Dev loop, checks, PR expectations |
| [../CHANGELOG.md](../CHANGELOG.md) | User-facing release notes |
| [../nix/README.md](../nix/README.md) | Flake attrs, Home Manager, `vendorHash` |
| [../packaging/aur/README.md](../packaging/aur/README.md) | AUR PKGBUILD publish checklist |
| [../contrib/systemd/README.md](../contrib/systemd/README.md) | User socket unit install |
| [../scripts/ci/consumer-smoke.sh](../scripts/ci/consumer-smoke.sh) | Offline resolve JSON + examples smoke |
| [../scripts/ci/aur-publish-check.sh](../scripts/ci/aur-publish-check.sh) | AUR PKGBUILD publish readiness |

## Drift rules

When you change resolve behavior, packaging, or the public contract:

1. Update the row’s source-of-truth doc first.
2. Touch every page that links that topic (at least README + this index if you add/remove a doc).
3. Keep [consumer-contract.md](consumer-contract.md), [resolve-result.schema.json](resolve-result.schema.json), and [resolve-batch-result.schema.json](resolve-batch-result.schema.json) aligned.
4. Run `make check-docs-crosslinks` (also part of `make check` / CI).
5. Extend unit/CLI/MCP/daemon tests for new public surfaces; keep `make check-consumer-smoke` green.

Issue forms and security contact links live under [`.github/ISSUE_TEMPLATE/`](../.github/ISSUE_TEMPLATE/) and point back here / SECURITY.
