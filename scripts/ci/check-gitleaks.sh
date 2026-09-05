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
  trap 'rm -rf -- "$tmp"' EXIT
  curl --fail --silent --show-error --location \
    --retry 3 --retry-all-errors --retry-max-time 180 \
    --connect-timeout 15 --max-time 120 \
    --output "$tmp/gitleaks.tar.gz" "$url"
  tar -xzf "$tmp/gitleaks.tar.gz" -C "$tmp"
  mv "$tmp/gitleaks" "$bin"
  chmod +x "$bin"
  rm -rf "$tmp"
  trap - EXIT
fi

"$bin" detect --source "$ROOT" --verbose --redact
