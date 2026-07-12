# Example: desktop notification with an appicon-resolved icon.
#
# Requires: notify-send, appicon on PATH.
# Usage:   bash examples/notify-appicon.sh [query] [summary] [body]

set -euo pipefail

APPICON_BIN="${APPICON_BIN:-appicon}"
query="${1:-firefox}"
summary="${2:-appicon}"
body="${3:-Resolved via appicon resolve — local path only.}"

if ! command -v notify-send >/dev/null 2>&1; then
  echo "notify-send not found" >&2
  exit 1
fi
if ! command -v "$APPICON_BIN" >/dev/null 2>&1; then
  echo "appicon not found on PATH" >&2
  exit 1
fi

icon=$("$APPICON_BIN" resolve --format png --size 48 "$query" 2>/dev/null || true)
if [[ -n "${icon:-}" ]]; then
  notify-send -i "$icon" "$summary" "$body"
else
  notify-send "$summary" "$body (no icon for ${query})"
fi
