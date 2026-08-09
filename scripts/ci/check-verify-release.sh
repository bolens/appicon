#!/usr/bin/env bash
# Test release verification success, completeness, and corruption handling.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
fixture=$(mktemp -d "${TMPDIR:-/tmp}/appicon-verify-check.XXXXXX")
trap 'rm -rf "$fixture"' EXIT

printf 'one' > "$fixture/one.tar.gz"
printf 'two' > "$fixture/two.tar.gz"
(cd "$fixture" && sha256sum one.tar.gz two.tar.gz > SHA256SUMS)
bash "$ROOT/scripts/ci/verify-release.sh" "$fixture" >/dev/null

# Stock macOS provides shasum rather than GNU sha256sum. Use an isolated PATH
# to exercise that branch with the host's real shasum implementation.
fallback_bin="$fixture/fallback-bin"
mkdir -p "$fallback_bin"
shasum_path=$(command -v shasum)
ln -s "$shasum_path" "$fallback_bin/shasum"
PATH="$fallback_bin" /bin/bash "$ROOT/scripts/ci/verify-release.sh" "$fixture" >/dev/null

if PATH="$fixture/empty-path" /bin/bash "$ROOT/scripts/ci/verify-release.sh" "$fixture" >/dev/null 2>&1; then
  echo "release verification accepted a missing checksum tool" >&2
  exit 1
fi

mv "$fixture/two.tar.gz" "$fixture/two.missing"
if bash "$ROOT/scripts/ci/verify-release.sh" "$fixture" >/dev/null 2>&1; then
  echo "release verification accepted a missing archive" >&2
  exit 1
fi
mv "$fixture/two.missing" "$fixture/two.tar.gz"

printf 'corrupt' > "$fixture/one.tar.gz"
if bash "$ROOT/scripts/ci/verify-release.sh" "$fixture" >/dev/null 2>&1; then
  echo "release verification accepted a corrupt archive" >&2
  exit 1
fi
echo "ok: release verification is portable and rejects incomplete/corrupt assets"
