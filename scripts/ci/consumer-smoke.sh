#!/usr/bin/env bash
# Consumer smoke: validate resolve --json shape + examples syntax (no live network).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

BIN="${APPICON_BIN:-}"
if [[ -z "$BIN" ]]; then
  make -s build
  BIN="$ROOT/bin/appicon"
fi

SCHEMA="$ROOT/docs/resolve-result.schema.json"
BATCH_SCHEMA="$ROOT/docs/resolve-batch-result.schema.json"
QUERIES="$ROOT/testdata/consumer/dock-queries.txt"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-$(mktemp -d)}"
export XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-$(mktemp -d)}"
export APPICON_NO_DAEMON=1

# Fixture XDG roots when present.
if [[ -d "$ROOT/testdata/xdg/share" ]]; then
  export XDG_DATA_DIRS="$ROOT/testdata/xdg/share${XDG_DATA_DIRS:+:$XDG_DATA_DIRS}"
fi

python3 - "$BIN" "$SCHEMA" "$BATCH_SCHEMA" "$QUERIES" <<'PY'
import json, subprocess, sys
from pathlib import Path

bin_path, schema_path, batch_schema_path, queries_path = sys.argv[1:5]
schema = json.loads(Path(schema_path).read_text())
batch_schema = json.loads(Path(batch_schema_path).read_text())
required = set(schema.get("required", []))
batch_required = set(batch_schema.get("required", []))

def check_obj(obj, label):
    missing = required - set(obj)
    if missing:
        raise SystemExit(f"{label}: missing keys {sorted(missing)}")
    for k in ("query", "path", "source", "theme", "format", "cached", "error"):
        if k not in obj:
            raise SystemExit(f"{label}: missing {k}")

queries = [ln.strip() for ln in Path(queries_path).read_text().splitlines() if ln.strip() and not ln.startswith("#")]
if not queries:
    raise SystemExit("no dock queries")

# Single-query miss must still emit a valid object.
proc = subprocess.run([bin_path, "resolve", "--json", "--offline", "--local", "zzzz-consumer-smoke-miss"], capture_output=True, text=True)
obj = json.loads(proc.stdout)
check_obj(obj, "miss")
if proc.returncode not in (0, 1):
    raise SystemExit(f"unexpected miss exit {proc.returncode}")

# Batch envelope.
proc = subprocess.run([bin_path, "resolve", "--json", "--offline", "--local", *queries[:3]], capture_output=True, text=True)
batch = json.loads(proc.stdout)
missing_batch = batch_required - set(batch)
if missing_batch:
    raise SystemExit(f"batch missing keys {sorted(missing_batch)}")
if "results" not in batch or not isinstance(batch["results"], list) or not batch["results"]:
    raise SystemExit("batch missing results")
for i, item in enumerate(batch["results"]):
    check_obj(item, f"batch[{i}]")

# Glyph hit (supported path).
proc = subprocess.run([bin_path, "resolve", "--json", "--offline", "--local", "--order", "glyph", "smoke-app"], capture_output=True, text=True)
obj = json.loads(proc.stdout)
check_obj(obj, "glyph")
if proc.returncode != 0 or obj.get("source") != "glyph" or not obj.get("path"):
    raise SystemExit(f"glyph resolve failed: rc={proc.returncode} obj={obj}")

# Completions / man mention new surfaces (binary embeds scripts).
comp = subprocess.run([bin_path, "completion", "bash"], capture_output=True, text=True, check=True)
for needle in ("__complete queries", "suggest", "--from-desktop"):
    if needle not in comp.stdout:
        raise SystemExit(f"bash completion missing {needle!r}")

print(f"consumer-smoke ok ({len(queries)} dock queries checked)")
PY

# Shell examples must parse.
for f in "$ROOT"/examples/*.sh; do
  bash -n "$f"
done
echo "examples syntax ok"
