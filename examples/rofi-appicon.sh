# Example: Rofi window / app picker using appicon for icons.
#
# Requires: rofi, appicon on PATH.
# Usage:   bash examples/rofi-appicon.sh
#
# Shells out to `appicon resolve` only — no SVGL URLs here.

set -euo pipefail

APPICON_BIN="${APPICON_BIN:-appicon}"
SIZE="${APPICON_SIZE:-48}"
THEME="${APPICON_THEME:-dark}"

if ! command -v "$APPICON_BIN" >/dev/null 2>&1; then
  echo "appicon not found on PATH (set APPICON_BIN or run: make install / go install)" >&2
  exit 1
fi
if ! command -v rofi >/dev/null 2>&1; then
  echo "rofi not found on PATH" >&2
  exit 1
fi

# Demo entries: display name → resolve query
entries=(
  "Firefox|firefox"
  "Discord|discord"
  "Terminal|org.kde.konsole"
)

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

menu=()
for row in "${entries[@]}"; do
  label="${row%%|*}"
  query="${row#*|}"
  icon_path=""
  if path=$("$APPICON_BIN" resolve --format png --size "$SIZE" --theme "$THEME" "$query" 2>/dev/null); then
    icon_path=$path
  fi
  if [[ -n "$icon_path" ]]; then
    menu+=("${label}\0icon\x1f${icon_path}")
  else
    menu+=("${label}")
  fi
done

printf '%b\n' "${menu[@]}" | rofi -dmenu -i -p "appicon" -show-icons
