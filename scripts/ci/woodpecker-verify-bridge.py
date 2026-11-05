#!/usr/bin/env python3
"""Verify a native draft before the retained GitHub OIDC signing bridge."""

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
from urllib.parse import urlparse


def api(path):
    return json.loads(subprocess.check_output(["gh", "api", path], text=True))


def validate(directory, repo, tag, sha):
    manifest = json.loads((directory / "release-manifest.json").read_text())
    if (manifest["repository"], manifest["tag"], manifest["commit"]) != (
        repo,
        tag,
        sha,
    ):
        raise ValueError("native receipt differs from tagged source")
    builder = urlparse(manifest["builder"])
    if builder.scheme != "https" or builder.hostname != "woodpecker.bolens.dev":
        raise ValueError("unexpected native builder")
    expected = manifest["assets"]
    required = {
        f"appicon_{tag}_{osname}_{arch}.tar.gz"
        for osname in ["linux", "darwin", "windows"]
        for arch in ["amd64", "arm64"]
    }
    required |= {
        "appicon-packs-bundle.tar.gz",
        "SHA256SUMS",
        "woodpecker-provenance.json",
    }
    if set(expected) != required or set(expected) != {
        p.name for p in directory.iterdir()
    } - {"release-manifest.json"}:
        raise ValueError("draft asset set differs from native contract")
    for name, digest in expected.items():
        path = directory / name
        if (
            Path(name).name != name
            or not re.fullmatch(r"[0-9a-f]{64}", digest)
            or path.is_symlink()
            or not path.is_file()
        ):
            raise ValueError("invalid asset manifest")
        if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            raise ValueError("draft asset digest differs from native receipt")
    # Parse the untrusted checksum file without permitting paths, duplicates or omissions.
    checksums = {}
    for line in (directory / "SHA256SUMS").read_text().splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9_.-]+)", line)
        if not match or match[2] in checksums:
            raise ValueError("unsafe or duplicate checksum entry")
        checksums[match[2]] = match[1]
    if checksums != {
        name: digest for name, digest in expected.items() if name != "SHA256SUMS"
    }:
        raise ValueError("checksum subjects differ from native receipt")


def main():
    repo = os.environ["GITHUB_REPOSITORY"]
    tag = os.environ["GITHUB_REF_NAME"]
    sha = os.environ["GITHUB_SHA"]
    if (
        repo != "bolens/appicon"
        or os.environ["GITHUB_EVENT_NAME"] != "workflow_dispatch"
    ):
        raise ValueError("bridge requires explicit canonical GitHub dispatch")
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        raise ValueError("bridge must run at a release tag")
    reference = api(f"repos/{repo}/git/ref/tags/{tag}")["object"]
    if reference["type"] != "tag":
        raise ValueError("bridge requires signed annotated tag")
    signed = api(f"repos/{repo}/git/tags/{reference['sha']}")
    if (
        not signed["verification"]["verified"]
        or signed["object"]["type"] != "commit"
        or signed["object"]["sha"] != sha
    ):
        raise ValueError("tag signature/source verification failed")
    release = api(f"repos/{repo}/releases/tags/{tag}")
    if not release["draft"] or release["target_commitish"] != sha:
        raise ValueError("bridge requires exact-source native draft")
    directory = Path(".woodpecker-out/bridge")
    directory.mkdir(parents=True)
    subprocess.run(
        ["gh", "release", "download", tag, "--repo", repo, "--dir", str(directory)],
        check=True,
    )
    validate(directory, repo, tag, sha)
    print(
        "Verified native source, asset identity and checksums; ready for OIDC signing."
    )


if __name__ == "__main__":
    main()
