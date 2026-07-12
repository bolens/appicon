# Example: walker-style launcher stub using appicon PNG paths.
#
# walker (https://github.com/abenz1267/walker) configs vary by version.
# This script only demonstrates resolving icons to local PNG paths that you
# can plug into a custom walker/rofi/tofi source — no SVGL URLs.
#
# Usage: bash examples/walker-appicon.sh [query...]

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

for q in "${queries[@]}"; do
  if path=$("$APPICON_BIN" resolve --json --format png --size "$SIZE" --theme "$THEME" "$q" 2>/dev/null); then
    printf '%s\n' "$path"
  else
    printf '{"query":%s,"path":null,"error":"not found"}\n' "$(printf '%s' "$q" | sed 's/"/\\"/g; s/^/"/; s/$/"/')"
  fi
done
