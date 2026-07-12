#!/usr/bin/env bash
# Assert flake / AUR / Nix package versions and checksums stay aligned.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FLAKE="$ROOT/flake.nix"
PACKAGES_NIX="$ROOT/nix/packages.nix"
AUR="$ROOT/packaging/aur"

fail=0

hex_to_sri() {
  python3 -c "
import base64, binascii, sys
h = binascii.unhexlify(sys.argv[1].strip())
print('sha256-' + base64.b64encode(h).decode())
" "$1"
}

flake_ver="$(python3 - "$FLAKE" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
m = re.search(r'(?m)^\s*version\s*=\s*"([^"]+)";', text)
if not m:
    raise SystemExit("FAIL: flake.nix version = \"…\" not found")
print(m.group(1))
PY
)"
echo "flake version: $flake_ver"

pkgver_of() {
  local file="$1"
  sed -nE 's/^pkgver=([^[:space:]]+).*/\1/p' "$file" | head -1
}

srcinfo_pkgver() {
  local file="$1"
  sed -nE 's/^[[:space:]]*pkgver = ([^[:space:]]+).*/\1/p' "$file" | head -1
}

sha256_of() {
  # PKGBUILD: sha256sums=('hex') or sha256sums_x86_64=('hex')
  local file="$1" key="$2"
  python3 - "$file" "$key" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
key = sys.argv[2]
m = re.search(rf"(?m)^{re.escape(key)}=\('([0-9a-fA-F]{{64}})'\)", text)
if not m:
    raise SystemExit(f"FAIL: {key} missing in {sys.argv[1]}")
print(m.group(1).lower())
PY
}

sri_of_nix() {
  local system="$1"
  python3 - "$PACKAGES_NIX" "$system" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
system = sys.argv[2]
m = re.search(rf'{re.escape(system)}\s*=\s*"(sha256-[^"]+)";', text)
if not m:
    raise SystemExit(f"FAIL: nix binHashes.{system} missing")
print(m.group(1))
PY
}

# --- stable packages: flake == AUR pkgver == .SRCINFO ---
for pkg in appicon appicon-bin; do
  pb="$AUR/$pkg/PKGBUILD"
  si="$AUR/$pkg/.SRCINFO"
  pv="$(pkgver_of "$pb")"
  if [ "$pv" != "$flake_ver" ]; then
    echo "FAIL: $pkg PKGBUILD pkgver=$pv want $flake_ver" >&2
    fail=1
  else
    echo "PASS: $pkg PKGBUILD pkgver=$pv"
  fi
  if [ -f "$si" ]; then
    siv="$(srcinfo_pkgver "$si")"
    if [ "$siv" != "$pv" ]; then
      echo "FAIL: $pkg .SRCINFO pkgver=$siv want $pv" >&2
      fail=1
    else
      echo "PASS: $pkg .SRCINFO pkgver=$siv"
    fi
  else
    echo "FAIL: missing $si" >&2
    fail=1
  fi
done

# --- appicon-git: must have pkgver() and placeholder rooted at flake version ---
git_pb="$AUR/appicon-git/PKGBUILD"
if ! grep -qE '^pkgver\(\)' "$git_pb"; then
  echo "FAIL: appicon-git PKGBUILD missing pkgver()" >&2
  fail=1
else
  echo "PASS: appicon-git has pkgver()"
fi
git_pv="$(pkgver_of "$git_pb")"
case "$git_pv" in
  "$flake_ver".r* | "$flake_ver")
    echo "PASS: appicon-git placeholder pkgver=$git_pv"
    ;;
  *)
    echo "FAIL: appicon-git pkgver=$git_pv should start with ${flake_ver}.r… (refresh before AUR push)" >&2
    fail=1
    ;;
esac

# --- source archive checksum: AUR appicon sha256sums ---
src_hex="$(sha256_of "$AUR/appicon/PKGBUILD" sha256sums)"
echo "AUR appicon source sha256: $src_hex"

# --- binary checksums: AUR ↔ Nix SRI ---
amd_hex="$(sha256_of "$AUR/appicon-bin/PKGBUILD" sha256sums_x86_64)"
arm_hex="$(sha256_of "$AUR/appicon-bin/PKGBUILD" sha256sums_aarch64)"
amd_sri="$(hex_to_sri "$amd_hex")"
arm_sri="$(hex_to_sri "$arm_hex")"
nix_amd="$(sri_of_nix x86_64-linux)"
nix_arm="$(sri_of_nix aarch64-linux)"

if [ "$amd_sri" != "$nix_amd" ]; then
  echo "FAIL: amd64 hash mismatch AUR→SRI=$amd_sri nix=$nix_amd" >&2
  fail=1
else
  echo "PASS: amd64 AUR sha256 ↔ nix binHashes"
fi
if [ "$arm_sri" != "$nix_arm" ]; then
  echo "FAIL: arm64 hash mismatch AUR→SRI=$arm_sri nix=$nix_arm" >&2
  fail=1
else
  echo "PASS: arm64 AUR sha256 ↔ nix binHashes"
fi

# --- CHANGELOG mentions this version ---
if ! grep -qE "^## \[${flake_ver}\]" "$ROOT/CHANGELOG.md"; then
  echo "FAIL: CHANGELOG.md missing ## [$flake_ver]" >&2
  fail=1
else
  echo "PASS: CHANGELOG has [$flake_ver]"
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "ok: packaging versions aligned (flake=$flake_ver)"
