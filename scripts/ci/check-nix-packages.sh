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

for needle in 'appicon-bin' 'appicon-git' 'packages.nix'; do
  if ! grep -q "$needle" "$FLAKE"; then
    echo "FAIL: flake.nix missing ${needle}" >&2
    fail=1
  fi
done

if ! grep -q 'daemon.enable' "$ROOT/nix/home-manager.nix"; then
  echo "FAIL: home-manager.nix missing programs.appicon.daemon.enable" >&2
  fail=1
else
  echo "PASS: HM daemon.enable"
fi

if ! grep -q 'environmentFiles' "$ROOT/nix/home-manager.nix"; then
  echo "FAIL: home-manager.nix missing environmentFiles (BYOK EnvironmentFile)" >&2
  fail=1
else
  echo "PASS: HM environmentFiles"
fi

# Guard against the sops secret-path footgun: no active assignment of
# config.sops.secrets.*.path into environment (comments/docs may warn about it).
if grep -E '^\s+[A-Za-Z_][A-Za-Z0-9_]*\s*=\s*config\.sops\.secrets\.' "$ROOT/nix/home-manager.nix"; then
  echo "FAIL: home-manager.nix must not assign config.sops.secrets.* into environment (use environmentFiles + templates)" >&2
  fail=1
else
  echo "PASS: HM no sops.secrets assignment in environment"
fi

if ! grep -q 'lib/systemd/user' "$PACKAGES_NIX"; then
  echo "FAIL: packages.nix should install systemd user units" >&2
  fail=1
else
  echo "PASS: systemd user units in packages.nix"
fi

# Binary package must pin release checksums (linux amd64 + arm64).
if ! grep -q 'sha256-W7PxOUoQAXKY3gBh3Cb+Ij3DDeGL03S0Q3ln2Rt1tFo=' "$PACKAGES_NIX"; then
  echo "FAIL: appicon-bin amd64 hash missing/outdated in nix/packages.nix" >&2
  fail=1
fi
if ! grep -q 'sha256-TH3K5BFYrsZD6S5SXTa7x7zvNp3qjQw19460ihMvb6g=' "$PACKAGES_NIX"; then
  echo "FAIL: appicon-bin arm64 hash missing/outdated in nix/packages.nix" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "ok: nix package contract"
