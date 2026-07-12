# Example: Rofi window / app picker using appicon for icons.
#
# Requires: rofi, appicon on PATH, python3 (batch JSON).
# Usage:   bash examples/rofi-appicon.sh
#
# Shells out to `appicon resolve` only — no SVGL URLs here.
# Resolves the dock list in one batch call.

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

queries=()
labels=()
for row in "${entries[@]}"; do
  labels+=("${row%%|*}")
  queries+=("${row#*|}")
done

# Batch may exit 1 on partial miss; JSON is still on stdout.
batch=$("$APPICON_BIN" resolve --json --format png --size "$SIZE" --theme "$THEME" "${queries[@]}" 2>/dev/null || true)

tmp=$(mktemp)
trap 'rm -f "$tmp" "$tmp.queries" "$tmp.json"' EXIT
printf '%s\n' "${queries[@]}" >"$tmp.queries"
printf '%s' "$batch" >"$tmp.json"

mapfile -t icon_paths < <(python3 - "$tmp.queries" "$tmp.json" <<'PY'
import json, sys
queries = [ln.strip() for ln in open(sys.argv[1]) if ln.strip()]
raw = open(sys.argv[2]).read().strip()
by_q = {}
if raw:
    data = json.loads(raw)
    items = data.get("results") or [data]
    for it in items:
        by_q[it.get("query")] = it.get("path") or ""
for q in queries:
    print(by_q.get(q, ""))
PY
)

menu=()
for i in "${!labels[@]}"; do
  label="${labels[$i]}"
  icon_path="${icon_paths[$i]:-}"
  if [[ -n "$icon_path" ]]; then
    menu+=("${label}\0icon\x1f${icon_path}")
  else
    menu+=("${label}")
  fi
done

printf '%b\n' "${menu[@]}" | rofi -dmenu -i -p "appicon" -show-icons
