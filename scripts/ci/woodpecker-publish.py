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


_coordinator_token = None
_coordinator_request = None


def authorize_coordinator():
    """Accept only the protected signed-tag GitHub workflow, never a PR credential."""
    sha = os.environ.get("GITHUB_SHA", "")
    tag = os.environ.get("GITHUB_REF_NAME", "")
    expected = {
        "GITHUB_ACTIONS": "true",
        "GITHUB_REPOSITORY": REPOSITORY,
        "GITHUB_EVENT_NAME": "workflow_dispatch",
        "GITHUB_REF_TYPE": "tag",
        "GITHUB_REF": f"refs/tags/{tag}",
        "GITHUB_WORKFLOW_REF": f"{REPOSITORY}/.github/workflows/release.yml@refs/tags/{tag}",
        "WOODPECKER_PUBLISHING": "true",
    }
    if any(os.environ.get(key) != value for key, value in expected.items()):
        raise ValueError(
            "merge coordination requires the canonical enabled signed-tag workflow"
        )
    if not re.fullmatch(r"[0-9a-f]{40}", sha) or not re.fullmatch(
        r"v[0-9]+\.[0-9]+\.[0-9]+", tag
    ):
        raise ValueError("invalid coordinator release source")
    if run("git", "rev-parse", "HEAD") != sha or run(
        "git", "status", "--porcelain", "--untracked-files=no"
    ):
        raise ValueError("coordinator checkout differs from the signed-tag source")
    request = {
        "repository": REPOSITORY,
        "commit": sha,
        "tag": tag,
        "pr": os.environ.get("PACKAGING_PR", ""),
        "head": os.environ.get("PACKAGING_HEAD", ""),
        "base": os.environ.get("PACKAGING_BASE", ""),
        "tree": os.environ.get("PACKAGING_TREE", ""),
    }
    if not re.fullmatch(r"[1-9][0-9]*", request["pr"]) or any(
        not re.fullmatch(r"[0-9a-f]{40}", request[key])
        for key in ["head", "base", "tree"]
    ):
        raise ValueError("invalid immutable packaging merge intent")
    identity = os.environ.get("WOODPECKER_PUBLISHER_ID", "")
    token = os.environ.get("GITHUB_COORDINATOR_TOKEN", "")
    if (
        not re.fullmatch(r"[1-9][0-9]*", identity)
        or not token
        or any(ord(c) < 33 or ord(c) > 126 for c in token)
    ):
        raise ValueError("coordinator publisher identity or platform token unavailable")
    global _coordinator_token, _coordinator_request
    _coordinator_token = os.environ.pop("GITHUB_COORDINATOR_TOKEN")
    _coordinator_request = request
    return request


BROKER_URL = "https://woodpecker.bolens.dev/publisher/credentials"
_cached_token = None
_cached_expiration = None
_mints = 0


def clean_environment():
    blocked = {
        "GH_TOKEN",
        "GITHUB_TOKEN",
        "GITHUB_COORDINATOR_TOKEN",
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
    if _coordinator_token is not None:
        return _coordinator_token
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
        headers={"Content-Type": "application/json", "User-Agent": "woodpecker-publisher/1"},
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
    if event == "push" and (
        os.environ.get("CI_COMMIT_BRANCH") != "main"
        or tag
        or os.environ.get("CI_COMMIT_REF") != "refs/heads/main"
    ):
        raise ValueError("publication denied: Pages requires main push")
    return sha, tag


def prepared_version():
    """Source release intent is independent of last-published package hashes."""
    match = re.fullmatch(r"([0-9]+\.[0-9]+\.[0-9]+)\n?", Path("VERSION").read_text())
    if match is None:
        raise ValueError("VERSION must contain exactly one stable X.Y.Z version")
    return match[1]


def verify_tag_source(sha, tag):
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
    version = prepared_version()
    if tag != f"v{version}":
        raise ValueError("release tag differs from the prepared source version")


def guard(event: str) -> tuple[str, str]:
    sha, tag = context(event)
    # Host setting is the single-publisher cutover, defaulting safely to disabled.
    setting = api(f"repos/{REPOSITORY}/actions/variables/WOODPECKER_PUBLISHING")
    if setting["value"] != "true":
        raise PublicationDisabled("Woodpecker publication has not been enabled")
    if event == "tag":
        verify_tag_source(sha, tag)
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
    if release.get("author", {}).get("id") != approved_publisher_id():
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


def verify_signed_checksums(directory, record):
    """Bind packaging hashes to the retained tag-specific keyless signing identity."""
    run(
        "cosign",
        "verify-blob",
        "--bundle",
        str(directory / "SHA256SUMS.sigstore.json"),
        "--certificate-identity",
        f"https://github.com/{REPOSITORY}/.github/workflows/release.yml@refs/tags/{record['tag']}",
        "--certificate-oidc-issuer",
        "https://token.actions.githubusercontent.com",
        str(directory / "SHA256SUMS"),
    )
    expected = {
        name: digest
        for name, digest in record["assets"].items()
        if name != "SHA256SUMS"
    }
    signed = {}
    for line in (directory / "SHA256SUMS").read_text().splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9_.-]+)", line)
        if not match or match[2] in signed:
            raise ValueError("unsafe or duplicate signed checksum subject")
        signed[match[2]] = match[1]
    if signed != expected:
        raise ValueError(
            "packaging hashes differ from the tag-signed checksum subjects"
        )


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
        if REPOSITORY == "bolens/appicon" and _coordinator_request is not None:
            download(record["tag"], "SHA256SUMS.sigstore.json", Path(directory))
            verify_signed_checksums(Path(directory), record)


def approved_publisher_id():
    if _coordinator_request is not None:
        return int(os.environ["WOODPECKER_PUBLISHER_ID"])
    identity = api(f"repos/{REPOSITORY}/actions/variables/WOODPECKER_PUBLISHER_ID")[
        "value"
    ]
    if not re.fullmatch(r"[1-9][0-9]*", identity):
        raise ValueError("invalid approved publisher identity")
    return int(identity)


def validate_packaging_pr(
    pr, branch, base_sha, repository_id, publisher_id, require_base=True
):
    """A branch-name match alone never authorizes resuming or merging a PR."""
    for side, ref in [("head", branch), ("base", "main")]:
        repo = pr.get(side, {}).get("repo") or {}
        if (
            repo.get("id") != repository_id
            or repo.get("full_name") != REPOSITORY
            or pr[side].get("ref") != ref
        ):
            raise ValueError(
                "packaging PR is not a canonical repository/base/head binding"
            )
    if pr.get("user", {}).get("id") != publisher_id:
        raise ValueError("packaging PR author is not the approved publisher")
    if type(pr.get("number")) is not int or pr["number"] < 1:
        raise ValueError("invalid packaging PR number")
    if require_base and pr["base"].get("sha") != base_sha:
        raise ValueError(
            "packaging PR is stale against main; update and revalidate it through normal review"
        )
    if not re.fullmatch(r"[0-9a-f]{40}", pr["head"].get("sha", "")):
        raise ValueError("invalid packaging PR source")
    if pr.get("state") != "open" and not pr.get("merged_at"):
        raise ValueError(
            "packaging PR was closed without merge; inspect before recovery"
        )
    return pr["head"]["sha"]


def verify_packaging_tree(head_sha, base_sha, expected_tree, paths):
    run("git", "fetch", "--no-tags", f"https://github.com/{REPOSITORY}.git", head_sha)
    if run("git", "rev-parse", "FETCH_HEAD^{tree}") != expected_tree:
        raise ValueError(
            "packaging PR tree differs from the freshly generated candidate"
        )
    changed = set(run("git", "diff", "--name-only", base_sha, head_sha).splitlines())
    if changed - set(paths):
        raise ValueError(
            "packaging PR contains changes outside the generated-file allowlist"
        )


def validate_coordinator_intent(intent, base_sha, expected_tree, prior, changed):
    if expected_tree != intent["tree"] or (
        base_sha != intent["base"]
        and not (prior and prior[0].get("merged_at") and not changed)
    ):
        raise ValueError(
            "coordinator generated tree/base differs from the native merge intent"
        )
    if (
        len(prior) != 1
        or str(prior[0]["number"]) != intent["pr"]
        or prior[0]["head"]["sha"] != intent["head"]
    ):
        raise ValueError("coordinator PR/head differs from the native merge intent")


def packaging():
    if _coordinator_request is not None:
        sha, tag = _coordinator_request["commit"], _coordinator_request["tag"]
    else:
        sha, tag = guard("tag")
    record = json.loads((OUTPUT / "release-manifest.json").read_text())
    if (record["commit"], record["tag"]) != (sha, tag):
        raise ValueError("packaging manifest belongs to another candidate")
    verify_assets(record)
    branch = f"chore/woodpecker-packaging-{tag}"
    repository_id = api(f"repos/{REPOSITORY}")["id"]
    publisher_id = approved_publisher_id()
    # Query canonical owner/base, then independently validate full repository IDs.
    owner = REPOSITORY.split("/")[0]
    prior = api(
        f"repos/{REPOSITORY}/pulls?state=all&head={owner}:{branch}&base=main&per_page=100"
    )
    if len(prior) > 1:
        raise ValueError("ambiguous packaging PR history; inspect before recovery")
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
        paths = [
            "flake.nix",
            "nix/packages.nix",
            "packaging/aur/appicon/PKGBUILD",
            "packaging/aur/appicon/.SRCINFO",
            "packaging/aur/appicon-bin/PKGBUILD",
            "packaging/aur/appicon-bin/.SRCINFO",
            "packaging/aur/appicon-git/PKGBUILD",
            "packaging/aur/appicon-git/.SRCINFO",
        ]
    else:
        paths = ["packaging/arch/PKGBUILD", "packaging/arch/.SRCINFO"]
    # Always regenerate on a fresh canonical base before trusting an existing PR.
    run(
        "git",
        "fetch",
        "--no-tags",
        f"https://github.com/{REPOSITORY}.git",
        "refs/heads/main",
    )
    base_sha = run("git", "rev-parse", "FETCH_HEAD")
    run("git", "checkout", "--detach", base_sha)
    current_version = prepared_version()
    if current_version != tag[1:]:
        raise ValueError(
            "main has advanced to another release version; do not downgrade packaging"
        )
    run(*command)
    changed = set(run("git", "diff", "--name-only", "HEAD").splitlines())
    if changed - set(paths):
        raise ValueError(
            "generator changed files outside its explicit packaging allowlist"
        )
    run("git", "add", "--", *paths)
    expected_tree = run("git", "write-tree")
    if _coordinator_request is not None:
        validate_coordinator_intent(
            _coordinator_request, base_sha, expected_tree, prior, changed
        )
    if prior:
        pr = prior[0]
        completed = bool(pr.get("merged_at")) and not changed
        head = validate_packaging_pr(
            pr,
            branch,
            base_sha,
            repository_id,
            publisher_id,
            require_base=not completed,
        )
        verify_packaging_tree(head, base_sha, expected_tree, paths)
        if pr.get("merged_at"):
            if changed:
                raise ValueError(
                    "previous packaging PR merged, but current packaging differs; use a new reviewed repair PR"
                )
            merged = pr.get("merge_commit_sha", "")
            if not re.fullmatch(r"[0-9a-f]{40}", merged):
                raise ValueError("merged packaging PR lacks an immutable merge receipt")
            run("git", "merge-base", "--is-ancestor", merged, base_sha)
            (OUTPUT / "packaging-receipt.json").write_text(
                json.dumps(
                    {
                        "status": "merged",
                        "repository": REPOSITORY,
                        "commit": sha,
                        "tag": tag,
                        "pr": str(pr["number"]),
                        "head": head,
                        "base": base_sha,
                        "tree": expected_tree,
                        "merge_commit": merged,
                    },
                    indent=2,
                )
                + "\n"
            )
            return
        if _coordinator_request is not None:
            finish_packaging(
                pr["number"], head, base_sha, branch, repository_id, publisher_id
            )
        else:
            request_packaging_merge(
                pr["number"],
                head,
                base_sha,
                expected_tree,
                branch,
                repository_id,
                publisher_id,
                sha,
                tag,
            )
        return
    if not changed:
        (OUTPUT / "packaging-receipt.json").write_text(
            json.dumps(
                {
                    "status": "packaging_current",
                    "repository": REPOSITORY,
                    "commit": sha,
                    "tag": tag,
                    "base": base_sha,
                    "tree": expected_tree,
                },
                indent=2,
            )
            + "\n"
        )
        return
    run("git", "checkout", "--no-track", "-b", branch)
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
    if run("git", "rev-parse", "HEAD^{tree}") != expected_tree:
        raise ValueError(
            "committed packaging tree differs from verified generated output"
        )
    remote = run(
        "git",
        "ls-remote",
        f"https://github.com/{REPOSITORY}.git",
        f"refs/heads/{branch}",
    )
    if remote:
        head = remote.split()[0]
        verify_packaging_tree(head, base_sha, expected_tree, paths)
    else:
        head = run("git", "rev-parse", "HEAD")
        push(branch)
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
    number = pr_url.rsplit("/", 1)[-1]
    if not re.fullmatch(r"[1-9][0-9]*", number):
        raise ValueError("host did not return an unambiguous packaging PR")
    pr = api(f"repos/{REPOSITORY}/pulls/{number}")
    if validate_packaging_pr(pr, branch, base_sha, repository_id, publisher_id) != head:
        raise ValueError("packaging PR head changed during creation")
    request_packaging_merge(
        number,
        head,
        base_sha,
        expected_tree,
        branch,
        repository_id,
        publisher_id,
        sha,
        tag,
    )


def request_packaging_merge(
    pr_number,
    head,
    base_sha,
    tree,
    branch,
    repository_id,
    publisher_id,
    release_sha,
    tag,
):
    """Hand off the wait to the forge so both native runners remain available."""
    current = api(f"repos/{REPOSITORY}/pulls/{pr_number}")
    if (
        validate_packaging_pr(current, branch, base_sha, repository_id, publisher_id)
        != head
    ):
        raise ValueError("packaging PR source changed before coordinator dispatch")
    if api(f"repos/{REPOSITORY}/commits/main")["sha"] != base_sha:
        raise ValueError("packaging base changed before coordinator dispatch")
    inputs = {
        "woodpecker_finalize": "true",
        "packaging_pr": str(pr_number),
        "packaging_head": head,
        "packaging_base": base_sha,
        "packaging_tree": tree,
    }
    if REPOSITORY == "bolens/appicon":
        inputs["woodpecker_bridge"] = "false"
    api(
        f"repos/{REPOSITORY}/actions/workflows/release.yml/dispatches",
        "POST",
        {"ref": tag, "inputs": inputs},
    )
    receipt = {
        "status": "packaging_pending",
        "repository": REPOSITORY,
        "commit": release_sha,
        "tag": tag,
        "pr": str(pr_number),
        "head": head,
        "base": base_sha,
        "tree": tree,
        "coordinator": f"https://github.com/{REPOSITORY}/actions/workflows/release.yml",
    }
    (OUTPUT / "packaging-receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
    print("Release assets verified; packaging merge requested, not yet completed.")


def finalize_packaging():
    request = authorize_coordinator()
    OUTPUT.mkdir(exist_ok=True)
    receipt = dict(request, status="packaging_pending")
    path = OUTPUT / "packaging-receipt.json"
    path.write_text(json.dumps(receipt, indent=2) + "\n")
    try:
        verify_tag_source(request["commit"], request["tag"])
        download(request["tag"], "release-manifest.json", OUTPUT)
        record = json.loads((OUTPUT / "release-manifest.json").read_text())
        if any(
            record.get(key) != request[key] for key in ["repository", "commit", "tag"]
        ):
            raise ValueError("published native receipt belongs to another source")
        packaging()
        state = api(f"repos/{REPOSITORY}/pulls/{request['pr']}")
        if not state.get("merged_at") or state["head"]["sha"] != request["head"]:
            raise ValueError("packaging merge completion was not verified")
        receipt.update(status="merged", merge_commit=state["merge_commit_sha"])
    except BaseException as error:
        receipt.update(status="failed", error_type=type(error).__name__)
        raise
    finally:
        path.write_text(json.dumps(receipt, indent=2) + "\n")


def finish_packaging(
    pr_number,
    expected_head: str,
    base_sha: str,
    branch: str,
    repository_id: int,
    publisher_id: int,
):
    """Wait for all required checks, then use the host's protected squash merge."""
    pr_url = str(pr_number)
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
                current = api(f"repos/{REPOSITORY}/pulls/{pr_number}")
                if (
                    validate_packaging_pr(
                        current, branch, base_sha, repository_id, publisher_id
                    )
                    != expected_head
                ):
                    raise ValueError(
                        "packaging PR source changed while waiting for checks"
                    )
                if api(f"repos/{REPOSITORY}/commits/main")["sha"] != base_sha:
                    raise ValueError(
                        "main advanced while packaging checks ran; revalidate on the new base"
                    )
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
    release = json.loads((OUTPUT / "site-release.json").read_text())
    if release.get("repository") != REPOSITORY or api(
        f"repos/{REPOSITORY}/releases/latest"
    )["tag_name"] != release.get("tag"):
        raise ValueError("site was built for a superseded published release")
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
        if api(f"repos/{REPOSITORY}/releases/latest")["tag_name"] != release["tag"]:
            raise ValueError("published release changed before the Pages push")
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
    elif args.phase == "finalize-packaging":
        finalize_packaging()
    elif args.phase == "pages":
        if not args.site:
            parser.error("pages requires --site")
        pages(args.site)


if __name__ == "__main__":
    try:
        main()
    except PublicationDisabled as disabled:
        print(disabled)
