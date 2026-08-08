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
echo "ok: release verification rejects incomplete/corrupt assets"
