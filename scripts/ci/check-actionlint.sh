#!/usr/bin/env bash
# Download actionlint (pinned) and lint GitHub Actions workflows.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

ACTIONLINT_VERSION="${ACTIONLINT_VERSION:-1.7.7}"
TOOLS_DIR="${ROOT}/.tools"
mkdir -p "$TOOLS_DIR"
bin="${TOOLS_DIR}/actionlint"

if [[ ! -x "$bin" ]]; then
  url="https://github.com/rhysd/actionlint/releases/download/v${ACTIONLINT_VERSION}/actionlint_${ACTIONLINT_VERSION}_linux_amd64.tar.gz"
  tmp="$(mktemp -d)"
  curl -fsSL "$url" | tar -xz -C "$tmp"
  mv "$tmp/actionlint" "$bin"
  chmod +x "$bin"
  rm -rf "$tmp"
fi

"$bin" -color .github/workflows/*.yml
