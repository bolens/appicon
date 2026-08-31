#!/usr/bin/env python3
"""Update stable packaging from verified release hashes."""

from __future__ import annotations

import argparse
import base64
import re
from pathlib import Path


def replace_one(path: Path, pattern: str, replacement: str) -> None:
    text = path.read_text(encoding="utf-8")
    updated, count = re.subn(pattern, replacement, text, count=1, flags=re.MULTILINE)
    if count != 1:
        raise SystemExit(f"expected one match in {path}: {pattern}")
    path.write_text(updated, encoding="utf-8")


def sri(hex_digest: str) -> str:
    return "sha256-" + base64.b64encode(bytes.fromhex(hex_digest)).decode()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("version")
    parser.add_argument("source_sha256")
    parser.add_argument("amd64_sha256")
    parser.add_argument("arm64_sha256")
    args = parser.parse_args()

    for name in ("source_sha256", "amd64_sha256", "arm64_sha256"):
        value = getattr(args, name).lower()
        if not re.fullmatch(r"[0-9a-f]{64}", value) or value == "0" * 64:
            raise SystemExit(f"invalid {name}: {value}")
        setattr(args, name, value)

    replace_one(Path("flake.nix"), r'^\s*version\s*=\s*"[^"]+";', f'      version = "{args.version}";')
    replace_one(Path("packaging/aur/appicon/PKGBUILD"), r"^pkgver=.*$", f"pkgver={args.version}")
    replace_one(
        Path("packaging/aur/appicon/PKGBUILD"),
        r"^sha256sums=\('[0-9a-fA-F]{64}'\)$",
        f"sha256sums=('{args.source_sha256}')",
    )
    replace_one(Path("packaging/aur/appicon-bin/PKGBUILD"), r"^pkgver=.*$", f"pkgver={args.version}")
    replace_one(
        Path("packaging/aur/appicon-bin/PKGBUILD"),
        r"^sha256sums_x86_64=\('[0-9a-fA-F]{64}'\)$",
        f"sha256sums_x86_64=('{args.amd64_sha256}')",
    )
    replace_one(
        Path("packaging/aur/appicon-bin/PKGBUILD"),
        r"^sha256sums_aarch64=\('[0-9a-fA-F]{64}'\)$",
        f"sha256sums_aarch64=('{args.arm64_sha256}')",
    )

    source_info = Path("packaging/aur/appicon/.SRCINFO")
    replace_one(source_info, r"^\s*pkgver = .*$", f"\tpkgver = {args.version}")
    replace_one(source_info, r"^\s*source = .*$", f"\tsource = appicon-{args.version}.tar.gz::https://github.com/bolens/appicon/archive/refs/tags/v{args.version}.tar.gz")
    replace_one(source_info, r"^\s*sha256sums = [0-9a-fA-F]{64}$", f"\tsha256sums = {args.source_sha256}")

    binary_info = Path("packaging/aur/appicon-bin/.SRCINFO")
    replace_one(binary_info, r"^\s*pkgver = .*$", f"\tpkgver = {args.version}")
    replace_one(binary_info, r"^\s*source_x86_64 = .*$", f"\tsource_x86_64 = https://github.com/bolens/appicon/releases/download/v{args.version}/appicon_v{args.version}_linux_amd64.tar.gz")
    replace_one(binary_info, r"^\s*sha256sums_x86_64 = [0-9a-fA-F]{64}$", f"\tsha256sums_x86_64 = {args.amd64_sha256}")
    replace_one(binary_info, r"^\s*source_aarch64 = .*$", f"\tsource_aarch64 = https://github.com/bolens/appicon/releases/download/v{args.version}/appicon_v{args.version}_linux_arm64.tar.gz")
    replace_one(binary_info, r"^\s*sha256sums_aarch64 = [0-9a-fA-F]{64}$", f"\tsha256sums_aarch64 = {args.arm64_sha256}")

    packages = Path("nix/packages.nix")
    replace_one(packages, r'^\s*x86_64-linux\s*=\s*"sha256-[^"]+";', f'    x86_64-linux = "{sri(args.amd64_sha256)}";')
    replace_one(packages, r'^\s*aarch64-linux\s*=\s*"sha256-[^"]+";', f'    aarch64-linux = "{sri(args.arm64_sha256)}";')

    # The VCS package's placeholder tracks the stable base version. Its pkgver()
    # function still derives the authoritative value when AUR builds it.
    git_pkg = Path("packaging/aur/appicon-git/PKGBUILD")
    replace_one(git_pkg, r"^pkgver=.*$", f"pkgver={args.version}")
    git_info = Path("packaging/aur/appicon-git/.SRCINFO")
    replace_one(git_info, r"^\s*pkgver = .*$", f"\tpkgver = {args.version}")
    replace_one(git_info, r"^\s*provides = appicon=.*$", f"\tprovides = appicon={args.version}")


if __name__ == "__main__":
    main()
