#!/usr/bin/env python3
"""Assemble a versioned GitHub Pages artifact."""

from __future__ import annotations

import argparse
import shutil
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=Path("site"))
    parser.add_argument("--output", type=Path, default=Path("_site"))
    parser.add_argument("--architecture", type=Path, required=True)
    parser.add_argument("--architecture-name", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    if args.output.exists():
        raise SystemExit(f"output already exists: {args.output}")
    if not args.version.strip():
        raise SystemExit("version must not be empty")

    shutil.copytree(args.source, args.output)
    architecture_dir = args.output / "docs" / "architecture"
    architecture_dir.mkdir(parents=True)
    shutil.copy2(args.architecture, architecture_dir / args.architecture_name)

    replaced = 0
    for page in args.output.rglob("*.html"):
        source = page.read_text(encoding="utf-8")
        updated = source.replace("__SITE_VERSION__", args.version.strip())
        replaced += source.count("__SITE_VERSION__")
        page.write_text(updated, encoding="utf-8")
    if replaced == 0:
        raise SystemExit("no __SITE_VERSION__ token found")
    (args.output / ".nojekyll").touch()
    print(f"pages-build: OK ({replaced} version references)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
