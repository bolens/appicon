# Example: walker-style launcher stub using appicon PNG paths.
#
# walker (https://github.com/abenz1267/walker) configs vary by version.
# This script only demonstrates resolving icons to local PNG paths that you
# can plug into a custom walker/rofi/tofi source — no SVGL URLs.
#
# Usage: bash examples/walker-appicon.sh [query...]
# Uses one batch resolve when multiple queries are given.

set -euo pipefail

APPICON_BIN="${APPICON_BIN:-appicon}"
SIZE="${APPICON_SIZE:-48}"
THEME="${APPICON_THEME:-dark}"

if ! command -v "$APPICON_BIN" >/dev/null 2>&1; then
  echo "appicon not found on PATH" >&2
  exit 1
fi

queries=("$@")
if [[ ${#queries[@]} -eq 0 ]]; then
  queries=(firefox discord)
fi

# Batch may exit 1 on partial miss; JSON still on stdout.
batch=$("$APPICON_BIN" resolve --json --format png --size "$SIZE" --theme "$THEME" "${queries[@]}" 2>/dev/null || true)
printf '%s' "$batch" | python3 -c '
import json, sys
raw = sys.stdin.read().strip()
if not raw:
    raise SystemExit(0)
data = json.loads(raw)
items = data.get("results") or [data]
for it in items:
    path = it.get("path")
    if path:
        print(path)
    else:
        q = json.dumps(it.get("query") or "")
        print("{\"query\":%s,\"path\":null,\"error\":\"not found\"}" % q)
'
