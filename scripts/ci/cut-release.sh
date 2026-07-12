#!/usr/bin/env bash
# Prepare cutting a release tag (does not push).
#
# Usage:
#   bash scripts/ci/cut-release.sh v0.1.1
#
# Then: git push origin main && git push origin v0.1.1
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

ver=${1:-}
if [[ -z "$ver" || "$ver" != v* ]]; then
  echo "usage: $0 vX.Y.Z" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree dirty; commit or stash first" >&2
  git status -sb >&2
  exit 1
fi

if git rev-parse "$ver" >/dev/null 2>&1; then
  echo "tag $ver already exists" >&2
  exit 1
fi

echo "== make check =="
make check

echo "== annotate tag $ver =="
git tag -a "$ver" -m "Release $ver

See CHANGELOG.md for details.
"

echo "Created local tag $ver at $(git rev-parse --short HEAD)"
echo "Push when ready:"
echo "  git push origin HEAD"
echo "  git push origin $ver"
echo "Release workflow builds archives, SHA256SUMS, and cosign bundle."
echo "Then update packaging/aur/*/PKGBUILD pkgver + sha256sums from the release."
