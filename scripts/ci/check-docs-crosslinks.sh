#!/usr/bin/env bash
# Fail if docs hub / README / sibling pages are missing required cross-links.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

fail=0
note() { printf '%s\n' "$*" >&2; }
bad() { note "FAIL: $*"; fail=1; }

HUB=docs/README.md
ROOT_README=README.md

[[ -f "$HUB" ]] || bad "missing $HUB"
[[ -f "$ROOT_README" ]] || bad "missing $ROOT_README"

# Root README must point at the docs hub.
if ! grep -qE '\[([^\]]*docs[^\]]*|Documentation)\]\(docs/README\.md\)|\]\(docs/README\.md\)' "$ROOT_README"; then
  if ! grep -qF 'docs/README.md' "$ROOT_README"; then
    bad "$ROOT_README must link to docs/README.md"
  fi
fi

# Hub must link every docs page + schema (except itself).
shopt -s nullglob
for f in docs/*.md docs/*.json; do
  base="${f#docs/}"
  [[ "$base" == "README.md" ]] && continue
  if ! grep -qF "](${base})" "$HUB" && ! grep -qF "](./${base})" "$HUB"; then
    bad "$HUB must link to ${base}"
  fi
done

# Every docs/*.md (except hub) must link back to the hub and name a sibling or README.
for f in docs/*.md; do
  [[ "$f" == "$HUB" ]] && continue
  if ! grep -qE '\]\(\.?/?README\.md\)|\]\(docs/README\.md\)' "$f"; then
    bad "$f must link to docs hub (README.md)"
  fi
  # Encourage cross-links: at least one other docs page or root README / SECURITY / AGENTS / CONTRIBUTING.
  if ! grep -qE '\]\((consumer-contract|sources|packs|deferred|resolve-result\.schema\.json|\.\./README|\.\./SECURITY|\.\./AGENTS|\.\./CONTRIBUTING)\.md' "$f" \
    && ! grep -qE '\]\((consumer-contract|sources|packs|deferred)\.md\)|\]\(resolve-result\.schema\.json\)' "$f"; then
    bad "$f must link to at least one sibling doc or root doc"
  fi
done

# Hub must link key root docs (anti-orphan for project-level pages).
for link in '../README.md' '../SECURITY.md' '../AGENTS.md' '../CONTRIBUTING.md' '../CHANGELOG.md' '../nix/README.md' '../packaging/aur/README.md' '../contrib/systemd/README.md'; do
  if ! grep -qF "](${link})" "$HUB"; then
    bad "$HUB must link to ${link}"
  fi
done

# Packaging READMEs should point back at the docs hub or root README.
for f in nix/README.md packaging/aur/README.md contrib/systemd/README.md; do
  [[ -f "$f" ]] || continue
  if ! grep -qE 'docs/README\.md|\.\./docs/README\.md|\]\(\.\./README\.md\)|\]\(../../README\.md\)' "$f"; then
    bad "$f must link to docs/README.md or the root README"
  fi
done

# Root project docs should mention the hub or consumer-contract (keeps agents/contributors in the map).
for f in AGENTS.md CONTRIBUTING.md SECURITY.md; do
  if ! grep -qF 'docs/README.md' "$f" && ! grep -qF 'docs/consumer-contract.md' "$f"; then
    bad "$f must link to docs/README.md or docs/consumer-contract.md"
  fi
done

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi
note "ok: docs cross-links present"
