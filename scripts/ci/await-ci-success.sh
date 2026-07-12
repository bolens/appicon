#!/usr/bin/env bash
# Poll once: succeed if CI passed for a commit (used by release.yml).
# Usage: await-ci-success.sh <owner/repo> <sha>
# Exit 0 on success, 1 on failed CI, 2 while waiting / no runs yet, 3 on usage error.
set -euo pipefail

REPO=${1:-}
SHA=${2:-}
if [[ -z "$REPO" || -z "$SHA" ]]; then
  echo "usage: $0 <owner/repo> <sha>" >&2
  exit 3
fi

json="$(gh run list --repo "$REPO" --workflow ci.yml --commit "$SHA" --limit 30 \
  --json databaseId,status,conclusion,event,displayTitle,url)"

export AWAIT_CI_JSON="$json"
python3 <<'PY'
import json
import os
import sys

runs = json.loads(os.environ["AWAIT_CI_JSON"])
if not runs:
    print("no CI runs yet for this commit")
    raise SystemExit(2)

ok = [
    r
    for r in runs
    if r.get("status") == "completed" and r.get("conclusion") == "success"
]
if ok:
    r = ok[0]
    print(f"ok: CI run {r['databaseId']} succeeded ({r.get('event')}) {r.get('url', '')}")
    raise SystemExit(0)

pending = [
    r
    for r in runs
    if r.get("status") in ("queued", "in_progress", "waiting", "requested", "pending")
]
if pending:
    print(f"waiting: {len(pending)} CI run(s) still in progress")
    raise SystemExit(2)

failed = [
    r
    for r in runs
    if r.get("status") == "completed"
    and r.get("conclusion") in ("failure", "cancelled", "timed_out")
]
if failed:
    for r in failed:
        print(
            f"FAIL: CI run {r['databaseId']} conclusion={r['conclusion']} {r.get('url', '')}",
            file=sys.stderr,
        )
    raise SystemExit(1)

print("waiting: no successful CI run yet")
raise SystemExit(2)
PY
