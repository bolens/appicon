#!/usr/bin/env bash
# Exercise cross-platform release archive names, contents, and version metadata.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
dist=$(mktemp -d "${TMPDIR:-/tmp}/appicon-release-check.XXXXXX")
extract=$(mktemp -d "${TMPDIR:-/tmp}/appicon-release-extract.XXXXXX")
trap 'rm -rf "$dist" "$extract"' EXIT

if bash scripts/ci/build-release.sh 9.8.7 "$dist" >/dev/null 2>&1; then
  echo "builder accepted a version without vX.Y.Z format" >&2
  exit 1
fi
if APPICON_RELEASE_TARGETS=invalid bash scripts/ci/build-release.sh v9.8.7 "$dist" >/dev/null 2>&1; then
  echo "builder accepted an invalid target" >&2
  exit 1
fi

APPICON_RELEASE_TARGETS="linux/amd64 darwin/arm64 windows/amd64" \
  bash scripts/ci/build-release.sh v9.8.7 "$dist"

(cd "$dist" && sha256sum --check SHA256SUMS)
for target in linux_amd64 darwin_arm64 windows_amd64; do
  archive="$dist/appicon_v9.8.7_${target}.tar.gz"
  [[ -f "$archive" ]] || { echo "missing $archive" >&2; exit 1; }
  listing=$(tar -tzf "$archive")
  for member in LICENSE README.md completions/appicon.bash man/man1/appicon.1; do
    grep -qx "$member" <<<"$listing" || { echo "$archive missing $member" >&2; exit 1; }
  done
done

grep -qx appicon <<<"$(tar -tzf "$dist/appicon_v9.8.7_darwin_arm64.tar.gz")" \
  || { echo "darwin archive missing appicon" >&2; exit 1; }
grep -qx appicon.exe <<<"$(tar -tzf "$dist/appicon_v9.8.7_windows_amd64.tar.gz")" \
  || { echo "windows archive missing appicon.exe" >&2; exit 1; }
if tar -tzf "$dist/appicon_v9.8.7_darwin_arm64.tar.gz" | grep -q '^contrib/'; then
  echo "darwin archive unexpectedly contains Linux systemd units" >&2
  exit 1
fi
grep -q '^contrib/systemd/appicon.service$' \
  <<<"$(tar -tzf "$dist/appicon_v9.8.7_linux_amd64.tar.gz")" \
  || { echo "linux archive missing systemd units" >&2; exit 1; }
tar -xzf "$dist/appicon_v9.8.7_windows_amd64.tar.gz" -C "$extract" appicon.exe
go version -m "$extract/appicon.exe" | grep -q 'Version=v9.8.7' \
  || { echo "windows binary missing embedded release version" >&2; exit 1; }
if APPICON_RELEASE_TARGETS=linux/amd64 bash scripts/ci/build-release.sh v9.8.7 "$dist" >/dev/null 2>&1; then
  echo "builder overwrote existing release artifacts" >&2
  exit 1
fi
echo "ok: portable release archives"
