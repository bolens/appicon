#!/usr/bin/env bash
# Assert CI push on.paths ⊇ union of dorny filter paths (so push CI can start).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CI_YML="$ROOT/.github/workflows/ci.yml"

python3 - "$CI_YML" <<'PY'
import fnmatch
import re
import sys
from pathlib import Path

text = Path(sys.argv[1]).read_text(encoding="utf-8")
fail = 0


def extract_list(after_pattern: str) -> set[str]:
    m = re.search(after_pattern, text, flags=re.S | re.M)
    if not m:
        raise SystemExit(f"FAIL: block not found for {after_pattern!r}")
    return set(re.findall(r"""^\s+- ['"]([^'"]+)['"]""", m.group(1), flags=re.M))


push = extract_list(r"(?m)^  push:\n.*?^    paths:\n((?:      - .+\n)+)")
push_pos = {p for p in push if not p.startswith("!")}


def dorny(name: str) -> set[str]:
    # Do not use DOTALL: path lines must stay single-line ([^\n]+), else one
    # entry swallows the rest of the filters block.
    m = re.search(
        rf"(?m)^            {re.escape(name)}:\n((?:              - [^\n]+\n)+)",
        text,
    )
    if not m:
        raise SystemExit(f"FAIL: dorny filter {name!r} not found")
    return set(re.findall(r"""^\s+- ['"]([^'"]+)['"]""", m.group(1), flags=re.M))


DORNY_FILTERS = (
    "go",
    "lint",
    "docs",
    "scripts_ci",
    "workflow",
    "nix",
    "packaging",
    "vuln",
)

filters = {name: dorny(name) for name in DORNY_FILTERS}

filt_block = re.search(r"(?ms)filters: \|\n((?:            .+\n)+)", text)
if not filt_block:
    raise SystemExit("FAIL: dorny filters: | block not found")
found_keys = re.findall(r"(?m)^            ([A-Za-z0-9_]+):\s*$", filt_block.group(1))
extra = sorted(set(found_keys) - set(DORNY_FILTERS))
missing_keys = sorted(set(DORNY_FILTERS) - set(found_keys))
if extra:
    print(f"FAIL: dorny filters not checked: {', '.join(extra)}", file=sys.stderr)
    fail = 1
if missing_keys:
    print(f"FAIL: expected dorny filters missing: {', '.join(missing_keys)}", file=sys.stderr)
    fail = 1


def push_covers(path: str) -> bool:
    """True if a positive push glob would match this dorny path pattern."""
    if path in push_pos:
        return True
    for g in push_pos:
        if g.endswith("/**"):
            root = g[:-3]
            if path == root or path.startswith(root + "/") or path == g:
                return True
            # dorny path is itself a glob under this root
            if path.startswith(root + "/") or fnmatch.fnmatch(path, g):
                return True
        elif "**" in g or "*" in g:
            if fnmatch.fnmatch(path, g):
                return True
            # push **/*.go covers dorny **/*.go
            if g == path:
                return True
        else:
            if path == g or fnmatch.fnmatch(path, g):
                return True
    return False


union: set[str] = set()
for paths in filters.values():
    union |= paths

uncovered = sorted(p for p in union if not push_covers(p))
if uncovered:
    print("FAIL: push on.paths does not cover dorny filter path(s):", file=sys.stderr)
    for p in uncovered:
        print(f"  - {p}", file=sys.stderr)
    fail = 1

if "cancel-in-progress: true" not in text:
    print("FAIL: concurrency cancel-in-progress: true missing", file=sys.stderr)
    fail = 1
if not re.search(r"(?m)^concurrency:\n", text):
    print("FAIL: concurrency block missing", file=sys.stderr)
    fail = 1

if "matrix:" not in text or "./cmd/appicon" not in text:
    print("FAIL: unit-tests package matrix missing", file=sys.stderr)
    fail = 1

if fail:
    sys.exit(1)
print("ok: push paths cover dorny filters; concurrency + matrix present")
for name, paths in sorted(filters.items()):
    print(f"  {name}: {len(paths)} path(s)")
PY
