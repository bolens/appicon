# Example: warm the cache from installed .desktop files, then suggest overrides for misses.
#
# Usage: bash examples/prefetch-and-suggest.sh
# Optional peer: misses and suggest candidates are supported outcomes.

set -euo pipefail

APPICON_BIN="${APPICON_BIN:-appicon}"

if ! command -v "$APPICON_BIN" >/dev/null 2>&1; then
  echo "appicon not found on PATH (set APPICON_BIN)" >&2
  exit 1
fi

echo "== prefetch --from-desktop (offline after first warm if you prefer) =="
"$APPICON_BIN" prefetch --from-desktop --json || true

echo
echo "== override suggest --from-misses =="
"$APPICON_BIN" override suggest --from-misses --json || true

echo
echo "Apply a candidate with: appicon override suggest --apply <query>"
echo "Or: appicon override set <query> <target>"
