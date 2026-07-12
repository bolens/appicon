#!/usr/bin/env bash
# Download gitleaks (pinned) and scan the git history.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

GITLEAKS_VERSION="${GITLEAKS_VERSION:-8.21.2}"
TOOLS_DIR="${ROOT}/.tools"
mkdir -p "$TOOLS_DIR"
bin="${TOOLS_DIR}/gitleaks"

if [[ ! -x "$bin" ]]; then
  url="https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz"
  tmp="$(mktemp -d)"
  curl -fsSL "$url" | tar -xz -C "$tmp"
  mv "$tmp/gitleaks" "$bin"
  chmod +x "$bin"
  rm -rf "$tmp"
fi

"$bin" detect --source "$ROOT" --verbose --redact
