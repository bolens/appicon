#!/usr/bin/env python3
"""Guard and deliver immutable GitHub releases from the native Woodpecker forge."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time

REPOSITORY = "bolens/appicon"
OUTPUT = Path(".woodpecker-out")


class PublicationDisabled(ValueError):
    """A reviewed migration has not selected this publisher yet."""


def run(*args: str, **kwargs):
    kwargs.setdefault("timeout", 1200)
    return subprocess.run(
        args, check=True, text=True, capture_output=True, **kwargs
    ).stdout.strip()


def api(path: str, method: str = "GET", data=None):
    args = ["gh", "api", "--method", method, path]
    if data is not None:
        args += ["--input", "-"]
    raw = run(*args, input=json.dumps(data) if data is not None else None, timeout=120)
    return json.loads(raw) if raw else None


def context(event: str) -> tuple[str, str]:
    expected = {
        "CI_FORGE_TYPE": "github",
        "CI_REPO": REPOSITORY,
        "CI_PIPELINE_EVENT": event,
        "CI_FORGE_URL": "https://github.com",
    }
    for key, value in expected.items():
        if os.environ.get(key) != value:
            raise ValueError(f"publication denied: unexpected {key}")
    sha = os.environ.get("CI_COMMIT_SHA", "")
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("publication denied: invalid commit")
    if run("git", "rev-parse", "HEAD") != sha:
        raise ValueError("publication denied: checkout differs from event")
    if run("git", "status", "--porcelain", "--untracked-files=no"):
        raise ValueError("publication denied: tracked source was modified")
    tag = os.environ.get("CI_COMMIT_TAG", "")
    if event == "tag" and not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        raise ValueError("publication denied: invalid release tag")
    if event == "push" and (os.environ.get("CI_COMMIT_BRANCH") != "main" or tag):
        raise ValueError("publication denied: Pages requires main push")
    return sha, tag


def guard(event: str) -> tuple[str, str]:
    sha, tag = context(event)
    # Host setting is the single-publisher cutover, defaulting safely to disabled.
    setting = api(f"repos/{REPOSITORY}/actions/variables/WOODPECKER_PUBLISHING")
    if setting["value"] != "true":
        raise PublicationDisabled("Woodpecker publication has not been enabled")
    if event == "tag":
        ref = api(f"repos/{REPOSITORY}/git/ref/tags/{tag}")["object"]
        if ref["type"] != "tag":
            raise ValueError("release tag must be signed and annotated")
        signed = api(f"repos/{REPOSITORY}/git/tags/{ref['sha']}")
        if (
            not signed["verification"]["verified"]
            or signed["object"]["type"] != "commit"
            or signed["object"]["sha"] != sha
        ):
            raise ValueError("release tag signature/source verification failed")
        relation = api(f"repos/{REPOSITORY}/compare/{sha}...main")
        if relation["status"] not in {"ahead", "identical"}:
            raise ValueError("tagged source is not on canonical main")
    elif api(f"repos/{REPOSITORY}/commits/main")["sha"] != sha:
        raise ValueError("superseded Pages deployment")
    return sha, tag


def manifest() -> dict:
    sha, tag = context("tag")
    files = sorted((OUTPUT / "release").iterdir())
    if not files or any(not f.is_file() or f.is_symlink() for f in files):
        raise ValueError("release must contain regular files")
    result = {
        "repository": REPOSITORY,
        "commit": sha,
        "tag": tag,
        "builder": os.environ["CI_PIPELINE_URL"],
        "assets": {f.name: hashlib.sha256(f.read_bytes()).hexdigest() for f in files},
    }
    (OUTPUT / "release-manifest.json").write_text(json.dumps(result, indent=2) + "\n")
    return result


def draft() -> dict:
    sha, tag = guard("tag")
    record = manifest()
    # Never reuse a version: concurrent runs and retries fail closed, without --clobber.
    releases = api(f"repos/{REPOSITORY}/releases?per_page=100")
    if any(r["tag_name"] == tag for r in releases):
        raise ValueError("release version already exists; inspect it before recovery")
    notes = (OUTPUT / "release-notes.md").read_text()
    release = api(
        f"repos/{REPOSITORY}/releases",
        "POST",
        {
            "tag_name": tag,
            "target_commitish": sha,
            "name": tag,
            "body": notes,
            "draft": True,
            "prerelease": False,
        },
    )
    run(
        "gh",
        "release",
        "upload",
        tag,
        "--repo",
        REPOSITORY,
        *(str(f) for f in sorted((OUTPUT / "release").iterdir())),
        str(OUTPUT / "release-manifest.json"),
    )
    if REPOSITORY == "bolens/appicon":
        api(
            f"repos/{REPOSITORY}/actions/workflows/release.yml/dispatches",
            "POST",
            {"ref": tag, "inputs": {"woodpecker_bridge": "true"}},
        )
        deadline = time.monotonic() + 1800
        while time.monotonic() < deadline:
            current = api(f"repos/{REPOSITORY}/releases/{release['id']}")
            if not current["draft"]:
                break
            time.sleep(10)
        else:
            raise TimeoutError(
                "OIDC bridge did not publish within 30 minutes; draft retained"
            )
    else:
        api(f"repos/{REPOSITORY}/releases/{release['id']}", "PATCH", {"draft": False})
    # Verify every uploaded asset against locally built bytes after publication.
    verify_assets(record)
    return record


def verify_assets(record: dict):
    release = api(f"repos/{REPOSITORY}/releases/tags/{record['tag']}")
    if release["draft"] or release["target_commitish"] != record["commit"]:
        raise ValueError("published source differs from the built candidate")
    import tempfile

    with tempfile.TemporaryDirectory() as directory:
        for name, expected in record["assets"].items():
            run(
                "gh",
                "release",
                "download",
                record["tag"],
                "--repo",
                REPOSITORY,
                "--pattern",
                name,
                "--dir",
                directory,
            )
            if (
                hashlib.sha256((Path(directory) / name).read_bytes()).hexdigest()
                != expected
            ):
                raise ValueError("published asset digest mismatch")


def packaging():
    sha, tag = guard("tag")
    record = json.loads((OUTPUT / "release-manifest.json").read_text())
    if (record["commit"], record["tag"]) != (sha, tag):
        raise ValueError("packaging manifest belongs to another candidate")
    verify_assets(record)
    # Keep generated packaging updates reviewable; never push or bypass main.
    import tempfile
    import urllib.request
    import shutil

    with tempfile.TemporaryDirectory() as directory:
        archive = Path(directory) / "source.tar.gz"
        with (
            urllib.request.urlopen(
                f"https://github.com/{REPOSITORY}/archive/refs/tags/{tag}.tar.gz",
                timeout=30,
            ) as response,
            archive.open("wb") as output,
        ):
            shutil.copyfileobj(response, output)
        source = hashlib.sha256(archive.read_bytes()).hexdigest()
    command = ["python3", "scripts/ci/update-release-packaging.py", tag[1:], source]
    if REPOSITORY == "bolens/appicon":
        command += [
            record["assets"][f"appicon_{tag}_linux_{arch}.tar.gz"]
            for arch in ["amd64", "arm64"]
        ]
    # Run the generator on fresh main, not the detached tag checkout.
    run("git", "fetch", "origin", "main")
    branch = f"chore/woodpecker-packaging-{tag}"
    run("git", "checkout", "--no-track", "-b", branch, "origin/main")
    run(*command)
    paths = (
        ["packaging", "flake.nix"]
        if REPOSITORY == "bolens/appicon"
        else ["packaging/arch"]
    )
    run("git", "add", "--", *paths)
    if not run("git", "diff", "--cached", "--name-only"):
        return
    run(
        "git",
        "-c",
        "user.name=Woodpecker release bot",
        "-c",
        "user.email=woodpecker@users.noreply.github.com",
        "commit",
        "-m",
        f"chore(packaging): refresh {tag} immutable release hashes",
    )
    push(branch)
    run(
        "gh",
        "pr",
        "create",
        "--repo",
        REPOSITORY,
        "--base",
        "main",
        "--head",
        branch,
        "--title",
        f"chore(packaging): refresh {tag} release hashes",
        "--body",
        f"Generated from signed tag {tag} ({sha}) and verified published asset hashes. Require current checks and protected squash merge.",
    )


def push(branch: str, directory: str | None = None):
    import base64

    token = os.environ["GH_TOKEN"]
    env = dict(
        os.environ,
        GIT_CONFIG_COUNT="1",
        GIT_CONFIG_KEY_0="http.https://github.com/.extraheader",
        GIT_CONFIG_VALUE_0="AUTHORIZATION: basic "
        + base64.b64encode(f"x-access-token:{token}".encode()).decode(),
    )
    run(
        "git",
        "push",
        f"https://github.com/{REPOSITORY}.git",
        f"HEAD:refs/heads/{branch}",
        cwd=directory,
        env=env,
    )


def pages(directory: str):
    sha, _ = guard("push")
    import shutil
    import tempfile

    source = Path(directory).resolve()
    if not (source / "index.html").is_file() or any(
        f.is_symlink() for f in source.rglob("*")
    ):
        raise ValueError("invalid generated Pages directory")
    with tempfile.TemporaryDirectory() as temporary:
        dest = Path(temporary) / "site"
        shutil.copytree(source, dest)
        (dest / ".nojekyll").touch()
        run("git", "init", "-b", "gh-pages", str(dest))
        # Preserve branch history: no force pushes, and fail on concurrent publication.
        remote = run(
            "git",
            "ls-remote",
            f"https://github.com/{REPOSITORY}.git",
            "refs/heads/gh-pages",
        )
        if remote:
            run(
                "git",
                "fetch",
                "--depth=1",
                f"https://github.com/{REPOSITORY}.git",
                "gh-pages",
                cwd=str(dest),
            )
            run("git", "reset", "--soft", "FETCH_HEAD", cwd=str(dest))
        run("git", "add", "--all", cwd=str(dest))
        run(
            "git",
            "-c",
            "user.name=Woodpecker Pages bot",
            "-c",
            "user.email=woodpecker@users.noreply.github.com",
            "commit",
            "--allow-empty",
            "-m",
            f"docs(pages): deploy {sha}",
            cwd=str(dest),
        )
        # Guard a second time immediately before changing the publishing branch.
        guard("push")
        push("gh-pages", str(dest))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "phase", choices=["guard", "manifest", "publish", "packaging", "pages"]
    )
    parser.add_argument("--site")
    args = parser.parse_args()
    if args.phase == "guard":
        guard("tag")
    elif args.phase == "manifest":
        manifest()
    elif args.phase == "publish":
        draft()
    elif args.phase == "packaging":
        packaging()
    elif args.phase == "pages":
        if not args.site:
            parser.error("pages requires --site")
        pages(args.site)


if __name__ == "__main__":
    try:
        main()
    except PublicationDisabled as disabled:
        print(disabled)
