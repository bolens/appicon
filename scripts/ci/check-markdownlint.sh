#!/usr/bin/env bash
# Lint markdown with markdownlint-cli via npx (pinned).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

MARKDOWNLINT_CLI_VERSION="${MARKDOWNLINT_CLI_VERSION:-0.44.0}"

if ! command -v npx >/dev/null 2>&1; then
  echo "npx not found; skip markdownlint" >&2
  exit 0
fi

npx --yes "markdownlint-cli@${MARKDOWNLINT_CLI_VERSION}" \
  README.md AGENTS.md CONTRIBUTING.md docs/**/*.md
