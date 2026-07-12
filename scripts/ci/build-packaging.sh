#!/usr/bin/env bash
# Build-smoke AUR-equivalent packages and/or Nix flake attrs.
#
# Env:
#   APPICON_SMOKE_PACKAGE=appicon|appicon-bin|appicon-git  (default: all AUR-style)
#   APPICON_BUILD_NIX=1   also run nix build for all flake attrs (or APPICON_NIX_ATTR)
#   APPICON_NIX_ATTR=appicon|appicon-git|appicon-bin      (single nix attr)
#   APPICON_SKIP_BIN=1    skip downloading the GitHub release for appicon-bin
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

FLAKE_VER="$(python3 - <<'PY'
import re
text = open("flake.nix", encoding="utf-8").read()
print(re.search(r'(?m)^\s*version\s*=\s*"([^"]+)";', text).group(1))
PY
)"
echo "packaging smoke for v${FLAKE_VER}"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

pkgver_of() {
  sed -nE 's/^pkgver=([^[:space:]]+).*/\1/p' "$1" | head -1
}

sha256_of() {
  python3 - "$1" "$2" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
key = sys.argv[2]
print(re.search(rf"(?m)^{re.escape(key)}=\('([0-9a-fA-F]{{64}})'\)", text).group(1).lower())
PY
}

want_pkg() {
  local name="$1"
  local sel="${APPICON_SMOKE_PACKAGE:-}"
  [ -z "$sel" ] || [ "$sel" = "all" ] || [ "$sel" = "$name" ]
}

build_appicon_source() {
  echo "::group::aur/appicon (source build)"
  local aur_ver
  aur_ver="$(pkgver_of packaging/aur/appicon/PKGBUILD)"
  [ "$aur_ver" = "$FLAKE_VER" ]
  mkdir -p "$workdir/appicon/bin"
  CGO_ENABLED=0 go build \
    -ldflags "-X github.com/bolens/appicon/internal/version.Version=v${aur_ver}" \
    -o "$workdir/appicon/bin/appicon" ./cmd/appicon
  local got
  got="$("$workdir/appicon/bin/appicon" version | tr -d '\r')"
  if [ "$got" != "v${aur_ver}" ]; then
    echo "FAIL: appicon version got=$got want=v${aur_ver}" >&2
    exit 1
  fi
  echo "PASS: appicon version=$got"
  # resolve with no args should fail cleanly (usage), not panic
  if "$workdir/appicon/bin/appicon" resolve 2>/dev/null; then
    echo "FAIL: appicon resolve with no args should exit non-zero" >&2
    exit 1
  fi
  echo "::endgroup::"
}

build_appicon_git() {
  echo "::group::aur/appicon-git (vcs-style build)"
  local rev git_ver got
  rev="$(git rev-parse --short=7 HEAD 2>/dev/null || echo dirty)"
  git_ver="${FLAKE_VER}-unstable-${rev}"
  mkdir -p "$workdir/appicon-git/bin"
  CGO_ENABLED=0 go build \
    -ldflags "-X github.com/bolens/appicon/internal/version.Version=v${git_ver}" \
    -o "$workdir/appicon-git/bin/appicon" ./cmd/appicon
  got="$("$workdir/appicon-git/bin/appicon" version | tr -d '\r')"
  if [ "$got" != "v${git_ver}" ]; then
    echo "FAIL: appicon-git version got=$got want=v${git_ver}" >&2
    exit 1
  fi
  echo "PASS: appicon-git version=$got"
  grep -q 'git+' packaging/aur/appicon-git/PKGBUILD
  grep -qE '^pkgver\(\)' packaging/aur/appicon-git/PKGBUILD
  echo "::endgroup::"
}

build_appicon_bin() {
  if [ "${APPICON_SKIP_BIN:-0}" = "1" ]; then
    echo "skip appicon-bin download (APPICON_SKIP_BIN=1)"
    return 0
  fi
  echo "::group::aur/appicon-bin (release tarball)"
  local arch asset_arch sum_key expect archive url got_sum bin_ver
  arch="$(uname -m)"
  case "$arch" in
    x86_64 | amd64)
      asset_arch=amd64
      sum_key=sha256sums_x86_64
      ;;
    aarch64 | arm64)
      asset_arch=arm64
      sum_key=sha256sums_aarch64
      ;;
    *)
      echo "FAIL: unsupported arch $arch for appicon-bin smoke" >&2
      exit 1
      ;;
  esac
  expect="$(sha256_of packaging/aur/appicon-bin/PKGBUILD "$sum_key")"
  archive="appicon_v${FLAKE_VER}_linux_${asset_arch}.tar.gz"
  url="https://github.com/bolens/appicon/releases/download/v${FLAKE_VER}/${archive}"
  mkdir -p "$workdir/appicon-bin"
  if ! curl -fsSL -o "$workdir/appicon-bin/$archive" "$url"; then
    if [ "${APPICON_REQUIRE_BIN:-0}" = "1" ]; then
      echo "FAIL: could not download $url (APPICON_REQUIRE_BIN=1)" >&2
      exit 1
    fi
    echo "WARN: release asset not published yet ($url); skipping appicon-bin smoke"
    echo "::endgroup::"
    return 0
  fi
  got_sum="$(sha256sum "$workdir/appicon-bin/$archive" | awk '{print $1}')"
  if [ "$got_sum" != "$expect" ]; then
    echo "FAIL: appicon-bin sha256 got=$got_sum want=$expect" >&2
    exit 1
  fi
  tar -xzf "$workdir/appicon-bin/$archive" -C "$workdir/appicon-bin"
  test -x "$workdir/appicon-bin/appicon"
  bin_ver="$("$workdir/appicon-bin/appicon" version | tr -d '\r')"
  if [ "$bin_ver" != "v${FLAKE_VER}" ]; then
    echo "FAIL: appicon-bin version got=$bin_ver want=v${FLAKE_VER}" >&2
    exit 1
  fi
  echo "PASS: appicon-bin archive+version ok ($bin_ver)"
  echo "::endgroup::"
}

if want_pkg appicon; then build_appicon_source; fi
if want_pkg appicon-git; then build_appicon_git; fi
if want_pkg appicon-bin; then build_appicon_bin; fi

if [ "${APPICON_BUILD_NIX:-0}" = "1" ] || [ -n "${APPICON_NIX_ATTR:-}" ]; then
  echo "::group::nix builds"
  if ! command -v nix >/dev/null 2>&1; then
    echo "FAIL: nix build requested but nix not on PATH" >&2
    exit 1
  fi
  attrs=()
  if [ -n "${APPICON_NIX_ATTR:-}" ]; then
    attrs=("${APPICON_NIX_ATTR}")
  else
    attrs=(appicon appicon-git appicon-bin)
  fi
  extra=(--extra-experimental-features "nix-command flakes")
  for attr in "${attrs[@]}"; do
    echo "nix build .#${attr}"
    nix "${extra[@]}" build ".#${attr}" -o "$workdir/nix-${attr}"
    test -x "$workdir/nix-${attr}/bin/appicon"
    echo "PASS: nix ${attr} → $("$workdir/nix-${attr}/bin/appicon" version | tr -d '\r')"
  done
  echo "::endgroup::"
fi

echo "ok: packaging build smoke"
