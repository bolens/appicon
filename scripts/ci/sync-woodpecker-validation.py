#!/usr/bin/env python3
"""Synchronize the credential-free prefix of the canonical Woodpecker workflow."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

MARKER = b"# Generated validation profile ends here; protected-ref publishers follow.\n"


def validation_bytes(source: bytes) -> bytes:
    if source.count(MARKER) != 1:
        raise ValueError("canonical workflow must have exactly one publication boundary")
    prefix, publishers = source.split(MARKER)
    names = re.findall(rb"(?m)^- name: ([^\r\n]+)$", publishers)
    if not publishers.startswith(b"- name: release-publish\n") or names != [
        b"release-publish", b"pages-publish"
    ]:
        raise ValueError("publication boundary must precede the contiguous publisher steps")
    if not re.search(rb"(?m)^steps:\s*$", prefix) or re.search(rb"from_secret", prefix, re.I):
        raise ValueError("validation profile must contain steps and no secret references")
    return prefix


def sync(root: Path, *, write: bool = False) -> None:
    canonical = root / ".woodpecker.yml"
    target = root / ".woodpecker-validation/woodpecker.yml"
    if canonical.is_symlink() or target.is_symlink() or target.parent.is_symlink():
        raise ValueError("workflow profile paths must not be symlinks")
    if target.parent.exists() and not target.parent.is_dir():
        raise ValueError("validation profile path must be a directory")
    if target.parent.is_dir() and any(entry.name != target.name for entry in target.parent.iterdir()):
        raise ValueError("validation profile directory contains unexpected entries")
    expected = validation_bytes(canonical.read_bytes())
    if write:
        target.parent.mkdir(exist_ok=True)
        target.write_bytes(expected)
    elif not target.is_file() or target.read_bytes() != expected:
        raise ValueError("validation profile is missing or stale; run sync-woodpecker-validation.py --write")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--check", action="store_true", help="verify without writing files")
    mode.add_argument("--write", action="store_true", help="regenerate the owned validation profile")
    args = parser.parse_args()
    try:
        sync(Path(__file__).resolve().parents[2], write=args.write)
    except (OSError, ValueError) as error:
        print(error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
