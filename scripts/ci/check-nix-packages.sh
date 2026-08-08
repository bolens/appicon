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
sops_assignment_re='^[[:space:]]+[A-Za-z_][A-Za-z0-9_]*[[:space:]]*=[[:space:]]*config\.sops\.secrets\.'
if grep -qE "$sops_assignment_re" "$ROOT/nix/home-manager.nix"; then
  echo "FAIL: home-manager.nix must not assign config.sops.secrets.* into environment (use environmentFiles + templates)" >&2
  fail=1
else
  grep_status=$?
  if [ "$grep_status" -gt 1 ]; then
    echo "FAIL: could not check home-manager.nix for sops secret assignments" >&2
    fail=1
  else
    echo "PASS: HM no sops.secrets assignment in environment"
  fi
fi

if ! grep -q 'lib/systemd/user' "$PACKAGES_NIX"; then
  echo "FAIL: packages.nix should install systemd user units" >&2
  fail=1
else
  echo "PASS: systemd user units in packages.nix"
fi

# Binary packages must pin syntactically valid SRI hashes. Cross-file equality
# with AUR release hashes is checked by check-packaging-versions.sh.
for system in x86_64-linux aarch64-linux; do
  if ! grep -qE "${system}[[:space:]]*=[[:space:]]*\"sha256-[A-Za-z0-9+/]{43}=\";" "$PACKAGES_NIX"; then
    echo "FAIL: appicon-bin ${system} SRI hash missing/invalid in nix/packages.nix" >&2
    fail=1
  else
    echo "PASS: appicon-bin ${system} SRI hash"
  fi
done

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "ok: nix package contract"
