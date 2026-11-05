#!/usr/bin/env bash
# Create and verify a local signed release tag; never push.
# Usage: bash scripts/ci/cut-release.sh vX.Y.Z FULL_VALIDATED_MAIN_SHA
set -euo pipefail

fail() { echo "$*" >&2; exit 1; }
[[ $# == 2 ]] || fail "usage: $0 vX.Y.Z FULL_VALIDATED_MAIN_SHA"
ver=$1
validated=$2
[[ "$ver" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail 'expected a stable vX.Y.Z tag'
[[ "$validated" =~ ^[0-9a-f]{40}$ ]] || fail 'expected the full validated main SHA'

root=$(cd -- "$(dirname -- "$0")/../.." && pwd -P)
cd -- "$root"
actual_root=$(git rev-parse --show-toplevel)
[[ "$(cd -- "$actual_root" && pwd -P)" == "$root" ]] || fail 'Git checkout root mismatch'
case "$(git config --get remote.origin.url)" in
  https://github.com/bolens/appicon|https://github.com/bolens/appicon.git|\
  git@github.com:bolens/appicon|git@github.com:bolens/appicon.git|\
  github.com:bolens/appicon|github.com:bolens/appicon.git|\
  ssh://git@github.com/bolens/appicon|ssh://git@github.com/bolens/appicon.git) ;;
  *) fail 'origin must identify the canonical GitHub appicon repository' ;;
esac

check_source() {
  [[ "$(git symbolic-ref --quiet --short HEAD)" == main ]] || fail 'check out local main before tagging'
  [[ "$(git rev-parse HEAD)" == "$validated" ]] || fail 'HEAD differs from the validated main SHA'
  [[ -z "$(git status --porcelain)" ]] || fail 'working tree must be clean before tagging'
  [[ "$(git rev-parse refs/remotes/origin/main)" == "$validated" ]] || fail 'origin/main has advanced; validate its exact SHA first'
  source_version=$(python3 - <<'PY'
import re
from pathlib import Path
match = re.fullmatch(r"([0-9]+\.[0-9]+\.[0-9]+)\n?", Path("VERSION").read_text())
if match is None:
    raise SystemExit('VERSION must contain exactly one stable X.Y.Z version')
print(match[1])
PY
  )
  [[ "$ver" == "v$source_version" ]] || fail 'tag differs from prepared VERSION'
}

check_absent_tag() {
  if git show-ref --verify --quiet "refs/tags/$ver"; then
    fail "local tag $ver already exists"
  else
    status=$?
    [[ $status == 1 ]] || fail 'could not inspect local tags'
  fi
  if git ls-remote --exit-code --tags origin "refs/tags/$ver" "refs/tags/$ver^{}" >/dev/null; then
    fail "remote tag $ver already exists"
  else
    status=$?
    [[ $status == 2 ]] || fail 'could not inspect remote tags'
  fi
}

git fetch --no-tags origin refs/heads/main:refs/remotes/origin/main
check_source
check_absent_tag
echo '== make check (exact-main CI remains a release prerequisite) =='
make check
# Checks may take time: refresh the remote and repeat every source/tag guard.
git fetch --no-tags origin refs/heads/main:refs/remotes/origin/main
check_source
check_absent_tag
git tag -s -m "Release $ver" -- "$ver" "$validated"
[[ "$(git cat-file -t "refs/tags/$ver")" == tag ]] || fail 'release tag is not annotated'
[[ "$(git rev-parse "$ver^{commit}")" == "$validated" ]] || fail 'release tag source mismatch'
git verify-tag "$ver"
echo "Created and verified local signed tag $ver at $validated"
echo 'After the authoritative exact-main CI and release review, push only this tag:'
echo "  git push origin refs/tags/$ver:refs/tags/$ver"
