#!/usr/bin/env bash
# Verify a release SHA256SUMS file with optional cosign keyless bundle.
#
# Usage:
#   bash scripts/ci/verify-release.sh /path/to/dir
# Expects in that directory: SHA256SUMS, archives listed therein,
# and optionally SHA256SUMS.sigstore.json
set -euo pipefail

dir=${1:-.}
cd "$dir"

if [[ ! -f SHA256SUMS ]]; then
  echo "missing SHA256SUMS in $dir" >&2
  exit 1
fi

sha256sum --check --ignore-missing SHA256SUMS

bundle=SHA256SUMS.sigstore.json
if [[ -f "$bundle" ]]; then
  if ! command -v cosign >/dev/null 2>&1; then
    echo "warning: $bundle present but cosign not on PATH; skipped signature check" >&2
    exit 0
  fi
  cosign verify-blob \
    --bundle "$bundle" \
    --certificate-identity-regexp '^https://github.com/bolens/appicon/\.github/workflows/release\.yml@refs/tags/v' \
    --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
    SHA256SUMS
  echo "cosign: SHA256SUMS signature ok"
else
  echo "note: no $bundle — checksum-only verification"
fi
