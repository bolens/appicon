#!/usr/bin/env bash
# Assert flake exports AUR-parity package attrs: appicon, appicon-bin, appicon-git.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FLAKE="$ROOT/flake.nix"
PACKAGES_NIX="$ROOT/nix/packages.nix"

fail=0

require_file() {
  if [ ! -f "$1" ]; then
    echo "FAIL: missing $1" >&2
    fail=1
  fi
}

require_file "$FLAKE"
require_file "$PACKAGES_NIX"

for name in appicon appicon-bin appicon-git; do
  if ! grep -qE "pname = \"${name}\"|${name} =" "$PACKAGES_NIX" "$FLAKE"; then
    echo "FAIL: package attr ${name} not found in nix packaging" >&2
    fail=1
  else
    echo "PASS: ${name} present"
  fi
done

# Overlay / packages wiring in flake.nix
for needle in 'appicon-bin' 'appicon-git' 'packages.nix'; do
  if ! grep -q "$needle" "$FLAKE"; then
    echo "FAIL: flake.nix missing ${needle}" >&2
    fail=1
  fi
done

# Binary package must pin release checksums (linux amd64 + arm64).
if ! grep -q 'sha256-QzKy4zvDnAlf0UVTRXF/U7zt3lpp1g/EmRZ0zirkOiU=' "$PACKAGES_NIX"; then
  echo "FAIL: appicon-bin amd64 hash missing/outdated in nix/packages.nix" >&2
  fail=1
fi
if ! grep -q 'sha256-F68XRxQ5itdy2sEWviuyFOQF2q/eg0ixNfPmLLr9zyc=' "$PACKAGES_NIX"; then
  echo "FAIL: appicon-bin arm64 hash missing/outdated in nix/packages.nix" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "ok: nix package contract"
