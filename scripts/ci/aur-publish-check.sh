#!/usr/bin/env bash
# Fail if AUR stable/bin PKGBUILDs still have SKIP checksums (not publish-ready).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fail=0

check_pkgbuild() {
  local f="$1"
  if grep -E "sha256sums(_[^=]*)?=\([^)]*SKIP" "$f" >/dev/null 2>&1; then
    echo "AUR publish check: SKIP checksum in $f" >&2
    fail=1
  fi
  if ! grep -q '^pkgver=' "$f"; then
    echo "AUR publish check: missing pkgver in $f" >&2
    fail=1
  fi
}

check_pkgbuild "$ROOT/packaging/aur/appicon/PKGBUILD"
check_pkgbuild "$ROOT/packaging/aur/appicon-bin/PKGBUILD"
# appicon-git may use SKIP for VCS — that is intentional; only require pkgver().
if ! grep -q 'pkgver()' "$ROOT/packaging/aur/appicon-git/PKGBUILD"; then
  echo "AUR publish check: appicon-git missing pkgver()" >&2
  fail=1
fi

for pkg in appicon appicon-bin appicon-git; do
  if [[ ! -f "$ROOT/packaging/aur/$pkg/.SRCINFO" ]]; then
    echo "AUR publish check: missing .SRCINFO for $pkg" >&2
    fail=1
  fi
done

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi
echo "AUR publish readiness ok (reference PKGBUILDs; push to aur.archlinux.org still manual — see packaging/aur/README.md)"
