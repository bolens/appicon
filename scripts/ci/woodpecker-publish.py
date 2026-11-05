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


BROKER_URL = "https://woodpecker.bolens.dev/publisher/credentials"
_cached_token = None
_cached_expiration = None
_mints = 0


def clean_environment():
    blocked = {
        "GH_TOKEN",
        "GITHUB_TOKEN",
        "GH_ENTERPRISE_TOKEN",
        "GITHUB_ENTERPRISE_TOKEN",
        "GH_DEBUG",
        "PUBLISHER_LEASE",
        "GITHUB_PUBLISH_TOKEN_EXPIRES_AT",
    }
    env = {
        key: value
        for key, value in os.environ.items()
        if key not in blocked
        and not key.startswith(
            (
                "GIT_TRACE",
                "GIT_CONFIG_",
                "GIT_CURL",
                "GIT_TRANSFER_TRACE",
                "CURL_TRACE",
                "CURL_VERBOSE",
                "CI_NETRC_",
            )
        )
    }
    env.update(GH_HOST="github.com", GH_PROMPT_DISABLED="1", GIT_TERMINAL_PROMPT="0")
    return env


def credentials(minimum=120):
    """Redeem the pipeline-bound lease; keep short-lived App tokens only in memory."""
    from datetime import datetime, timezone
    import secrets
    import urllib.request

    global _cached_token, _cached_expiration, _mints
    now = datetime.now(timezone.utc)
    if _cached_token and (_cached_expiration - now).total_seconds() >= max(
        120, minimum
    ):
        return _cached_token
    lease = os.environ.get("PUBLISHER_LEASE")
    sha = os.environ.get("CI_COMMIT_SHA", "")
    if not lease or not re.fullmatch(r"[0-9a-f]{40}", sha) or _mints >= 8:
        raise ValueError(
            "publisher lease unavailable, invalid source or mint budget exhausted"
        )

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, request, fp, code, message, headers, newurl):
            raise ValueError("publisher broker redirects are forbidden")

    request = urllib.request.Request(
        BROKER_URL,
        method="POST",
        data=json.dumps(
            {
                "lease": lease,
                "repository": REPOSITORY,
                "commit": sha,
                "purpose": "github",
                "request_id": secrets.token_hex(16),
            }
        ).encode(),
        headers={"Content-Type": "application/json"},
    )
    _mints += 1
    try:
        with urllib.request.build_opener(NoRedirect).open(
            request, timeout=30
        ) as response:
            result = json.load(response)
        expires = datetime.fromisoformat(result["expires_at"].replace("Z", "+00:00"))
        token = result["token"]
        if (
            not isinstance(token, str)
            or not token
            or any(ord(character) < 33 or ord(character) == 127 for character in token)
            or expires.tzinfo is None
            or (expires - now).total_seconds() < max(120, minimum)
        ):
            raise ValueError("broker returned an unusable short-lived credential")
    except Exception:
        raise RuntimeError("publication credential redemption failed") from None
    _cached_token, _cached_expiration = token, expires
    return token


def publisher_environment():
    env = clean_environment()
    env["GH_TOKEN"] = credentials()
    return env


def token_budget(seconds: int):
    credentials(seconds)


def run(*args: str, **kwargs):
    kwargs.setdefault("timeout", 1200)
    kwargs.setdefault(
        "env", publisher_environment() if args[0] == "gh" else clean_environment()
    )
    return subprocess.run(
        args, check=True, text=True, capture_output=True, **kwargs
    ).stdout.strip()


def api(path: str, method: str = "GET", data=None):
    token_budget(60)
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
    if event == "tag" and os.environ.get("CI_COMMIT_REF") != f"refs/tags/{tag}":
        raise ValueError("publication denied: release ref differs from tag")
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
        if REPOSITORY == "bolens/appicon":
            version = re.search(
                r'(?m)^\s*version\s*=\s*"([^"]+)";', Path("flake.nix").read_text()
            )[1]
        else:
            version = Path("VERSION").read_text().strip()
        if tag != f"v{version}":
            raise ValueError("release tag differs from the prepared source version")
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


def publisher_identity(release: dict):
    identity = api(f"repos/{REPOSITORY}/actions/variables/WOODPECKER_PUBLISHER_ID")[
        "value"
    ]
    if not re.fullmatch(r"[1-9][0-9]*", identity) or release.get("author", {}).get(
        "id"
    ) != int(identity):
        raise ValueError("release author differs from the approved publisher identity")


def download(tag: str, name: str, directory: Path):
    run(
        "gh",
        "release",
        "download",
        tag,
        "--repo",
        REPOSITORY,
        "--pattern",
        name,
        "--dir",
        str(directory),
    )


def resume_record(release: dict, local: dict) -> dict:
    """Verify matching drafts, preserving original native bytes and invocation identity."""
    import tempfile
    from urllib.parse import urlparse

    if release["target_commitish"] != local["commit"]:
        raise ValueError("existing release targets another source; no writes")
    publisher_identity(release)
    assets = {asset["name"] for asset in release["assets"]}
    with tempfile.TemporaryDirectory() as temporary:
        directory = Path(temporary)
        if "release-manifest.json" in assets:
            download(local["tag"], "release-manifest.json", directory)
            record = json.loads((directory / "release-manifest.json").read_text())
            if any(
                record.get(key) != local[key] for key in ["repository", "commit", "tag"]
            ):
                raise ValueError("existing native receipt belongs to another source")
            builder = urlparse(record["builder"])
            if (
                builder.scheme != "https"
                or builder.hostname != "woodpecker.bolens.dev"
                or set(record["assets"]) != set(local["assets"])
            ):
                raise ValueError(
                    "existing native receipt has an unexpected builder or asset set"
                )
        elif assets:
            raise ValueError(
                "existing draft has assets but no receipt; restore its original release-manifest.json before retrying"
            )
        else:
            record = local
        allowed = set(record["assets"]) | {"release-manifest.json"}
        if REPOSITORY == "bolens/appicon":
            allowed.add("SHA256SUMS.sigstore.json")
        if assets - allowed:
            raise ValueError("existing release contains unexpected assets; no writes")
        for name, expected in record["assets"].items():
            if Path(name).name != name or not re.fullmatch(r"[0-9a-f]{64}", expected):
                raise ValueError("unsafe native receipt asset")
            if name in assets:
                download(record["tag"], name, directory)
                actual = hashlib.sha256((directory / name).read_bytes()).hexdigest()
            else:
                if not release["draft"]:
                    raise ValueError(
                        "published release is incomplete; never repair public versions in place"
                    )
                path = OUTPUT / "release" / name
                actual = hashlib.sha256(path.read_bytes()).hexdigest()
            if actual != expected:
                raise ValueError(
                    f"immutable asset mismatch: {name}; recover original bytes from {record['builder']} and rerun, never replace assets"
                )
        # Save the verified original build identity for packaging and subsequent recovery.
        (OUTPUT / "release-manifest.json").write_text(
            json.dumps(record, indent=2) + "\n"
        )
        if "release-manifest.json" not in assets:
            run(
                "gh",
                "release",
                "upload",
                record["tag"],
                "--repo",
                REPOSITORY,
                str(OUTPUT / "release-manifest.json"),
            )
        for name in sorted(set(record["assets"]) - assets):
            run(
                "gh",
                "release",
                "upload",
                record["tag"],
                "--repo",
                REPOSITORY,
                str(OUTPUT / "release" / name),
            )
    return record


def draft() -> dict:
    sha, tag = guard("tag")
    token_budget(600)
    local = manifest()
    releases = api(f"repos/{REPOSITORY}/releases?per_page=100")
    release = next((r for r in releases if r["tag_name"] == tag), None)
    if release is None:
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
    record = resume_record(release, local)
    if release["draft"]:
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
                    "OIDC bridge timed out; retry with a fresh broker token to resume the verified draft"
                )
        else:
            api(
                f"repos/{REPOSITORY}/releases/{release['id']}",
                "PATCH",
                {"draft": False},
            )
    verify_assets(record)
    return record


def verify_assets(record: dict):
    release = api(f"repos/{REPOSITORY}/releases/tags/{record['tag']}")
    if release["draft"] or release["target_commitish"] != record["commit"]:
        raise ValueError("published source differs from the built candidate")
    publisher_identity(release)
    if REPOSITORY == "bolens/appicon" and "SHA256SUMS.sigstore.json" not in {
        asset["name"] for asset in release["assets"]
    }:
        raise ValueError(
            "published Appicon release lacks the required OIDC signature bundle"
        )
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
    branch = f"chore/woodpecker-packaging-{tag}"
    prior = json.loads(
        run(
            "gh",
            "pr",
            "list",
            "--repo",
            REPOSITORY,
            "--state",
            "all",
            "--head",
            branch,
            "--json",
            "url,state,headRefOid",
        )
    )
    if prior:
        if len(prior) != 1 or prior[0]["state"] == "CLOSED":
            raise ValueError(
                "packaging branch has ambiguous or closed PR history; inspect before recovery"
            )
        if prior[0]["state"] == "MERGED":
            return
        finish_packaging(prior[0]["url"], prior[0]["headRefOid"])
        return
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
    if REPOSITORY == "bolens/appicon":
        current_version = re.search(
            r'(?m)^\s*version\s*=\s*"([^"]+)";', Path("flake.nix").read_text()
        )[1]
    else:
        current_version = Path("VERSION").read_text().strip()
    if current_version != tag[1:]:
        raise ValueError(
            "main has advanced to another release version; do not downgrade packaging"
        )
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
    remote = run("git", "ls-remote", "origin", f"refs/heads/{branch}")
    if remote:
        run("git", "fetch", "origin", branch)
        if run("git", "rev-parse", "FETCH_HEAD^{tree}") != run(
            "git", "rev-parse", "HEAD^{tree}"
        ):
            raise ValueError(
                "existing packaging branch differs from candidate; inspect it rather than force-push"
            )
        expected_head = run("git", "rev-parse", "FETCH_HEAD")
    else:
        push(branch)
        expected_head = run("git", "rev-parse", "HEAD")
    pr_url = run(
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
    finish_packaging(pr_url, expected_head)


def finish_packaging(pr_url: str, expected_head: str):
    """Wait for all required checks, then use the host's protected squash merge."""
    deadline = time.monotonic() + 2700
    while time.monotonic() < deadline:
        token_budget(60)
        result = subprocess.run(
            [
                "gh",
                "pr",
                "checks",
                pr_url,
                "--repo",
                REPOSITORY,
                "--required",
                "--json",
                "bucket,name",
            ],
            text=True,
            capture_output=True,
            timeout=120,
            env=publisher_environment(),
        )
        if result.returncode not in {0, 1, 8}:
            raise RuntimeError("could not read required PR checks; no merge attempted")
        if (
            result.returncode == 1
            and not result.stdout.strip()
            and "no checks" not in result.stderr.lower()
        ):
            raise RuntimeError(
                "required check query failed; diagnose authentication/host failure before retrying"
            )
        if result.stdout.strip():
            checks = json.loads(result.stdout)
            if any(check["bucket"] in {"fail", "cancel"} for check in checks):
                raise ValueError(
                    "packaging checks failed; retain PR for a reviewed fix"
                )
            if (
                checks
                and any(check["bucket"] == "pass" for check in checks)
                and all(check["bucket"] in {"pass", "skipping"} for check in checks)
            ):
                run(
                    "gh",
                    "pr",
                    "merge",
                    pr_url,
                    "--repo",
                    REPOSITORY,
                    "--squash",
                    "--delete-branch",
                    "--match-head-commit",
                    expected_head,
                )
                state = json.loads(
                    run(
                        "gh",
                        "pr",
                        "view",
                        pr_url,
                        "--repo",
                        REPOSITORY,
                        "--json",
                        "state,mergeCommit",
                    )
                )
                if state["state"] != "MERGED" or not state["mergeCommit"]:
                    raise ValueError(
                        "packaging PR is not merged; no completion claimed"
                    )
                return
        time.sleep(10)
    raise TimeoutError(
        "packaging checks did not complete; retry the existing PR with a fresh broker token"
    )


def push(branch: str, directory: str | None = None):
    import base64

    token = credentials()
    env = dict(
        clean_environment(),
        GIT_CONFIG_NOSYSTEM="1",
        GIT_CONFIG_GLOBAL="/dev/null",
        GIT_CONFIG_COUNT="2",
        GIT_CONFIG_KEY_0="credential.helper",
        GIT_CONFIG_VALUE_0="",
        GIT_CONFIG_KEY_1="http.https://github.com/.extraheader",
        GIT_CONFIG_VALUE_1="AUTHORIZATION: basic "
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
