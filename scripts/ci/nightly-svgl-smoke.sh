#!/usr/bin/env bash
# Live SVGL smoke: resolve a few known titles against the network.
# Used by .github/workflows/nightly-svgl.yml — not part of make check.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
bin="${APPICON_BIN:-$root/bin/appicon}"

if [[ ! -x "$bin" ]]; then
  echo "missing binary: $bin (run make build)" >&2
  exit 1
fi

"$bin" version

for q in firefox discord; do
  echo "== resolve $q =="
  out=$("$bin" resolve --json --format svg "$q")
  printf '%s\n' "$out"
  printf '%s\n' "$out" | python3 -c '
import json, sys
payload = json.load(sys.stdin)
assert payload.get("path"), payload
assert payload.get("source") == "svgl", payload
print("ok", payload["path"])
'
done
