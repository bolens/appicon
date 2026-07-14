#!/usr/bin/env bash
# Live SVGL smoke: resolve a few known titles against the network.
# Used by .github/workflows/nightly-svgl.yml — not part of make check.
#
# --order svgl skips builtins (file/overrides/xdg). GitHub ubuntu-latest ships
# Firefox as an XDG icon, which would otherwise beat SVGL for "firefox".
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
bin="${APPICON_BIN:-$root/bin/appicon}"

if [[ ! -x "$bin" ]]; then
  echo "missing binary: $bin (run make build)" >&2
  exit 1
fi

"$bin" version

# Isolate cache so a warm personal cache cannot mask network failures.
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-$(mktemp -d)}"
# Force in-process resolve (same as --local): do not dial a developer daemon
# that might serve a different binary/cache than $bin.
export APPICON_NO_DAEMON=1

for q in firefox discord; do
  echo "== resolve $q =="
  # --local: belt-and-suspenders with APPICON_NO_DAEMON above.
  out=$("$bin" resolve --json --format svg --order svgl --local "$q")
  printf '%s\n' "$out"
  printf '%s\n' "$out" | python3 -c '
import json, sys
payload = json.load(sys.stdin)
assert payload.get("path"), payload
assert payload.get("source") == "svgl", payload
# Guard against silent PNG conversion if format handling changes.
assert payload.get("format") == "svg", payload
print("ok", payload["path"])
'
done
