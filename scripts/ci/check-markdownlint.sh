#!/usr/bin/env bash
# Lint markdown with markdownlint-cli (pinned in package.json / package-lock.json).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v npm >/dev/null 2>&1; then
  echo "npm not found; skip markdownlint" >&2
  exit 0
fi

if [[ ! -d node_modules/markdownlint-cli ]]; then
  npm ci --ignore-scripts
fi

./node_modules/.bin/markdownlint \
  README.md AGENTS.md CONTRIBUTING.md CHANGELOG.md SECURITY.md docs/**/*.md
