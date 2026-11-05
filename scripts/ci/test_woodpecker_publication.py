#!/usr/bin/env python3
"""Publication trust-boundary tests: no network or credentials required."""

import importlib.util
import os
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "publish", Path(__file__).with_name("woodpecker-publish.py")
)
publish = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publish)


class PublicationGuards(unittest.TestCase):
    def setUp(self):
        isolated = patch.dict(
            os.environ,
            {
                key: value
                for key, value in os.environ.items()
                if not key.startswith("GIT_")
            },
            clear=True,
        )
        isolated.start()
        self.addCleanup(isolated.stop)
        from datetime import datetime, timedelta, timezone

        token = patch.object(publish, "_cached_token", "fixture-only")
        expiration = patch.object(
            publish,
            "_cached_expiration",
            datetime.now(timezone.utc) + timedelta(hours=1),
        )
        token.start()
        expiration.start()
        self.addCleanup(token.stop)
        self.addCleanup(expiration.stop)
        self.sha = "a" * 40
        self.env = {
            "CI_FORGE_TYPE": "github",
            "CI_FORGE_URL": "https://github.com",
            "CI_REPO": publish.REPOSITORY,
            "CI_PIPELINE_EVENT": "tag",
            "CI_COMMIT_SHA": self.sha,
            "CI_COMMIT_TAG": "v1.2.3",
            "CI_COMMIT_REF": "refs/tags/v1.2.3",
        }

    def invoke(self, **changes):
        env = dict(self.env, **changes)
        with (
            patch.dict(os.environ, env, clear=True),
            patch.object(publish, "run", side_effect=[self.sha, ""]),
        ):
            return publish.context("tag")

    def test_valid_tag_context(self):
        self.assertEqual(self.invoke(), (self.sha, "v1.2.3"))

    def test_reject_other_forges(self):
        for forge in ["gitea", "forgejo", ""]:
            with self.subTest(forge=forge), self.assertRaises(ValueError):
                self.invoke(CI_FORGE_TYPE=forge)

    def test_reject_other_repository(self):
        with self.assertRaises(ValueError):
            self.invoke(CI_REPO="attacker/repository")

    def test_reject_pull_request_and_manual(self):
        for event in ["pull_request", "manual", "push", "deployment"]:
            with self.subTest(event=event), self.assertRaises(ValueError):
                self.invoke(CI_PIPELINE_EVENT=event)

    def test_reject_untrusted_url(self):
        with self.assertRaises(ValueError):
            self.invoke(CI_FORGE_URL="https://gitea.example")

    def test_reject_invalid_tag(self):
        for tag in ["latest", "v1.2.3;id", "v1.2", "refs/tags/v1.2.3", "v1.2.3-rc1"]:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                self.invoke(CI_COMMIT_TAG=tag)

    def test_reject_checkout_mismatch(self):
        with (
            patch.dict(os.environ, self.env, clear=True),
            patch.object(publish, "run", return_value="b" * 40),
            self.assertRaises(ValueError),
        ):
            publish.context("tag")

    def test_reject_dirty_source(self):
        with (
            patch.dict(os.environ, self.env, clear=True),
            patch.object(publish, "run", side_effect=[self.sha, " M VERSION"]),
            self.assertRaises(ValueError),
        ):
            publish.context("tag")

    def test_disabled_publisher_cannot_call_release_api(self):
        with (
            patch.object(publish, "context", return_value=(self.sha, "v1.2.3")),
            patch.object(publish, "api", return_value={"value": "false"}) as api,
            self.assertRaises(ValueError),
        ):
            publish.guard("tag")
        self.assertEqual(api.call_count, 1)

    def test_reject_lightweight_tag(self):
        with (
            patch.object(publish, "context", return_value=(self.sha, "v1.2.3")),
            patch.object(
                publish,
                "api",
                side_effect=[{"value": "true"}, {"object": {"type": "commit"}}],
            ),
            self.assertRaises(ValueError),
        ):
            publish.guard("tag")

    def test_reject_unsigned_tag(self):
        signed = {
            "verification": {"verified": False},
            "object": {"type": "commit", "sha": self.sha},
        }
        with (
            patch.object(publish, "context", return_value=(self.sha, "v1.2.3")),
            patch.object(
                publish,
                "api",
                side_effect=[
                    {"value": "true"},
                    {"object": {"type": "tag", "sha": "b" * 40}},
                    signed,
                ],
            ),
            self.assertRaises(ValueError),
        ):
            publish.guard("tag")

    def test_reject_tag_outside_main(self):
        signed = {
            "verification": {"verified": True},
            "object": {"type": "commit", "sha": self.sha},
        }
        with (
            patch.object(publish, "context", return_value=(self.sha, "v1.2.3")),
            patch.object(
                publish,
                "api",
                side_effect=[
                    {"value": "true"},
                    {"object": {"type": "tag", "sha": "b" * 40}},
                    signed,
                    {"status": "diverged"},
                ],
            ),
            self.assertRaises(ValueError),
        ):
            publish.guard("tag")

    def test_existing_release_never_replaced(self):
        release = {"target_commitish": "b" * 40}
        with patch.object(publish, "api") as api, self.assertRaises(ValueError):
            publish.resume_record(release, {"commit": self.sha})
        api.assert_not_called()

    def test_published_release_source_must_match(self):
        record = {"tag": "v1.2.3", "commit": self.sha}
        with (
            patch.object(
                publish,
                "api",
                return_value={"draft": False, "target_commitish": "b" * 40},
            ),
            self.assertRaises(ValueError),
        ):
            publish.verify_assets(record)

    def test_pages_feature_branch_denied(self):
        env = dict(
            self.env,
            CI_PIPELINE_EVENT="push",
            CI_COMMIT_TAG="",
            CI_COMMIT_BRANCH="feature",
        )
        with (
            patch.dict(os.environ, env, clear=True),
            patch.object(publish, "run", side_effect=[self.sha, ""]),
            self.assertRaises(ValueError),
        ):
            publish.context("push")

    def test_expired_cache_without_lease_denied(self):
        from datetime import datetime, timezone

        with (
            patch.object(
                publish, "_cached_expiration", datetime(2000, 1, 1, tzinfo=timezone.utc)
            ),
            self.assertRaises(ValueError),
        ):
            publish.token_budget(60)

    def test_wrong_release_author_denied(self):
        with (
            patch.object(publish, "api", return_value={"value": "42"}),
            self.assertRaises(ValueError),
        ):
            publish.publisher_identity({"author": {"id": 43}})

    def test_draft_resume_keeps_original_bytes_and_uploads_only_missing(self):
        import hashlib
        import json
        import tempfile

        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            (output / "release").mkdir()
            contents = {"archive.tar.gz": b"original", "SHA256SUMS": b"checksums"}
            hashes = {
                name: hashlib.sha256(data).hexdigest()
                for name, data in contents.items()
            }
            record = {
                "repository": publish.REPOSITORY,
                "commit": self.sha,
                "tag": "v1.2.3",
                "builder": "https://woodpecker.bolens.dev/repos/123/pipeline/4",
                "assets": hashes,
            }
            server = dict(
                contents, **{"release-manifest.json": json.dumps(record).encode()}
            )
            # The rebuilt archive differs. Its original bytes are already on the server and must not be replaced.
            (output / "release/archive.tar.gz").write_bytes(b"rebuilt")
            (output / "release/SHA256SUMS").write_bytes(contents["SHA256SUMS"])
            release = {
                "draft": True,
                "target_commitish": self.sha,
                "assets": [
                    {"name": "archive.tar.gz"},
                    {"name": "release-manifest.json"},
                ],
            }

            def download(tag, name, directory):
                (directory / name).write_bytes(server[name])

            with (
                patch.object(publish, "OUTPUT", output),
                patch.object(publish, "publisher_identity"),
                patch.object(publish, "download", side_effect=download),
                patch.object(publish, "run") as run,
            ):
                self.assertEqual(publish.resume_record(release, record), record)
            self.assertEqual(run.call_count, 1)
            self.assertEqual(run.call_args.args[-1], str(output / "release/SHA256SUMS"))

    def test_failed_required_check_cannot_merge(self):
        import subprocess

        result = subprocess.CompletedProcess(
            [], 1, '[{"bucket":"fail","name":"native"}]', ""
        )
        with (
            patch.object(subprocess, "run", return_value=result),
            patch.object(publish, "run") as run,
            self.assertRaises(ValueError),
        ):
            publish.finish_packaging(
                7, self.sha, "b" * 40, "chore/woodpecker-packaging-v1.2.3", 123, 42
            )
        run.assert_not_called()

    def test_passed_checks_use_exact_head_protected_squash(self):
        import subprocess

        result = subprocess.CompletedProcess(
            [], 0, '[{"bucket":"pass","name":"native"}]', ""
        )
        with (
            patch.object(subprocess, "run", return_value=result),
            patch.object(
                publish, "api", side_effect=[self.packaging_pr(), {"sha": "b" * 40}]
            ),
            patch.object(
                publish,
                "run",
                side_effect=["", '{"state":"MERGED","mergeCommit":{"oid":"a"}}'],
            ) as run,
        ):
            publish.finish_packaging(
                7, self.sha, "b" * 40, "chore/woodpecker-packaging-v1.2.3", 123, 42
            )
        command = run.call_args_list[0].args
        self.assertIn("--squash", command)
        self.assertNotIn("--admin", command)
        self.assertEqual(command[-2:], ("--match-head-commit", self.sha))

    def packaging_pr(self):
        repo = {"id": 123, "full_name": publish.REPOSITORY}
        return {
            "number": 7,
            "state": "open",
            "merged_at": None,
            "user": {"id": 42},
            "head": {
                "repo": dict(repo),
                "ref": "chore/woodpecker-packaging-v1.2.3",
                "sha": self.sha,
            },
            "base": {"repo": dict(repo), "ref": "main", "sha": "b" * 40},
        }

    def test_packaging_pr_canonical_binding(self):
        self.assertEqual(
            publish.validate_packaging_pr(
                self.packaging_pr(),
                "chore/woodpecker-packaging-v1.2.3",
                "b" * 40,
                123,
                42,
            ),
            self.sha,
        )

    def test_packaging_pr_fork_same_name_wrong_author_base_denied(self):
        import copy

        cases = []
        pr = self.packaging_pr()
        pr["head"]["repo"]["full_name"] = "fork/repository"
        cases.append(pr)
        pr = self.packaging_pr()
        pr["head"]["repo"]["id"] = 999
        cases.append(pr)
        pr = self.packaging_pr()
        pr["head"]["ref"] = "other"
        cases.append(pr)
        pr = self.packaging_pr()
        pr["user"]["id"] = 99
        cases.append(pr)
        pr = self.packaging_pr()
        pr["base"]["ref"] = "development"
        cases.append(pr)
        pr = self.packaging_pr()
        pr["base"]["repo"]["id"] = 999
        cases.append(pr)
        pr = self.packaging_pr()
        pr["base"]["sha"] = "c" * 40
        cases.append(pr)
        for pr in cases:
            with self.subTest(pr=copy.deepcopy(pr)), self.assertRaises(ValueError):
                publish.validate_packaging_pr(
                    pr, "chore/woodpecker-packaging-v1.2.3", "b" * 40, 123, 42
                )

    def test_packaging_pr_unrelated_tree_or_paths_denied(self):
        for outputs in [
            ["", "wrong-tree"],
            ["", "expected-tree", "unrelated/source.py"],
        ]:
            with (
                self.subTest(outputs=outputs),
                patch.object(publish, "run", side_effect=outputs),
                self.assertRaises(ValueError),
            ):
                publish.verify_packaging_tree(
                    self.sha, "b" * 40, "expected-tree", ["packaging/file"]
                )

    def test_half_cutover_retains_old_actions(self):
        import re

        path = Path(__file__).resolve().parents[2] / ".github/workflows/ci.yml"
        source = path.read_text()
        match = re.search(
            r"    if: (\(vars.WOODPECKER_LINUX_CI[^\n]+?vars.WOODPECKER_PUBLISHING != 'true'\))",
            source,
        )
        self.assertIsNotNone(match)
        guard = match[1]
        for linux, publisher, expected in [
            (False, False, True),
            (True, False, True),
            (False, True, True),
            (True, True, False),
        ]:
            expression = (
                guard.replace(
                    "vars.WOODPECKER_LINUX_CI", repr("true" if linux else "false")
                )
                .replace(
                    "vars.WOODPECKER_PUBLISHING", repr("true" if publisher else "false")
                )
                .replace("||", "or")
            )
            self.assertEqual(eval(expression, {"__builtins__": {}}, {}), expected)

    def test_pages_release_change_cannot_push_stale_metadata(self):
        import json
        import tempfile

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "site"
            source.mkdir()
            (source / "index.html").write_text("fixture")
            (root / "site-release.json").write_text(
                json.dumps({"repository": publish.REPOSITORY, "tag": "v1.2.3"})
            )
            with (
                patch.object(publish, "OUTPUT", root),
                patch.object(publish, "guard", return_value=(self.sha, "")),
                patch.object(
                    publish,
                    "api",
                    side_effect=[{"tag_name": "v1.2.3"}, {"tag_name": "v1.2.4"}],
                ),
                patch.object(publish, "run", return_value=""),
                patch.object(publish, "push") as push,
                self.assertRaises(ValueError),
            ):
                publish.pages(str(source))
            push.assert_not_called()

    def test_pages_preserves_history_and_removes_stale_files(self):
        import tempfile

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            remote = root / "pages.git"
            publish.run("git", "init", "--bare", str(remote))
            source = root / "output"
            source.mkdir()
            (source / "index.html").write_text("first")
            (source / "stale.html").write_text("removed on next deploy")
            original_run = publish.run

            def local_run(*args, **kwargs):
                args = tuple(
                    str(remote)
                    if arg == f"https://github.com/{publish.REPOSITORY}.git"
                    else arg
                    for arg in args
                )
                return original_run(*args, **kwargs)

            def local_push(branch, directory=None):
                original_run(
                    "git",
                    "push",
                    str(remote),
                    f"HEAD:refs/heads/{branch}",
                    cwd=directory,
                )

            import json

            (root / "site-release.json").write_text(
                json.dumps({"repository": publish.REPOSITORY, "tag": "v1.2.3"})
            )
            with (
                patch.object(publish, "OUTPUT", root),
                patch.object(publish, "api", return_value={"tag_name": "v1.2.3"}),
                patch.object(publish, "guard", return_value=(self.sha, "")),
                patch.object(publish, "run", side_effect=local_run),
                patch.object(publish, "push", side_effect=local_push),
            ):
                publish.pages(str(source))
                first = original_run(
                    "git", "--git-dir", str(remote), "rev-parse", "gh-pages"
                )
                (source / "stale.html").unlink()
                (source / "index.html").write_text("second")
                publish.pages(str(source))
                parent = original_run(
                    "git", "--git-dir", str(remote), "rev-parse", "gh-pages^"
                )
                self.assertEqual(parent, first)
                files = original_run(
                    "git",
                    "--git-dir",
                    str(remote),
                    "ls-tree",
                    "--name-only",
                    "gh-pages",
                ).splitlines()
                self.assertEqual(set(files), {".nojekyll", "index.html"})
                self.assertEqual(
                    original_run(
                        "git", "--git-dir", str(remote), "show", "gh-pages:index.html"
                    ),
                    "second",
                )


class NativeBridgeIntegrity(unittest.TestCase):
    def setUp(self):
        import tempfile

        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        spec = importlib.util.spec_from_file_location(
            "bridge", Path(__file__).with_name("woodpecker-verify-bridge.py")
        )
        self.bridge = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.bridge)
        self.sha = "a" * 40
        self.tag = "v1.2.3"
        names = {
            f"appicon_{self.tag}_{osname}_{arch}.tar.gz"
            for osname in ["linux", "darwin", "windows"]
            for arch in ["amd64", "arm64"]
        }
        names |= {"appicon-packs-bundle.tar.gz", "woodpecker-provenance.json"}
        for name in names:
            (self.directory / name).write_bytes(b"fixture")
        import hashlib

        sums = "".join(
            hashlib.sha256((self.directory / name).read_bytes()).hexdigest()
            + "  "
            + name
            + "\n"
            for name in sorted(names)
        )
        (self.directory / "SHA256SUMS").write_text(sums)
        self.refresh()

    def refresh(self):
        import hashlib
        import json

        self.record = {
            "repository": "bolens/appicon",
            "tag": self.tag,
            "commit": self.sha,
            "builder": "https://woodpecker.bolens.dev/repos/123/pipeline/4",
            "assets": {
                p.name: hashlib.sha256(p.read_bytes()).hexdigest()
                for p in self.directory.iterdir()
                if p.name != "release-manifest.json"
            },
        }
        (self.directory / "release-manifest.json").write_text(json.dumps(self.record))

    def verify(self):
        self.bridge.validate(self.directory, "bolens/appicon", self.tag, self.sha)

    def test_complete_matching_receipt(self):
        self.verify()

    def test_asset_tampering_denied(self):
        (self.directory / "woodpecker-provenance.json").write_bytes(b"tampered")
        with self.assertRaises(ValueError):
            self.verify()

    def test_wrong_source_denied(self):
        with self.assertRaises(ValueError):
            self.bridge.validate(self.directory, "bolens/appicon", self.tag, "b" * 40)

    def test_checksum_path_traversal_denied(self):
        with (self.directory / "SHA256SUMS").open("a") as sums:
            sums.write("a" * 64 + "  ../../outside\n")
        self.refresh()
        with self.assertRaises(ValueError):
            self.verify()

    def test_missing_platform_denied(self):
        (self.directory / f"appicon_{self.tag}_darwin_arm64.tar.gz").unlink()
        self.refresh()
        with self.assertRaises(ValueError):
            self.verify()


class PublisherLeaseTests(unittest.TestCase):
    def setUp(self):
        from datetime import datetime, timedelta, timezone

        self.datetime = datetime
        self.timedelta = timedelta
        self.timezone = timezone
        self.sha = "a" * 40
        patchers = [
            patch.dict(
                os.environ,
                {
                    "PUBLISHER_LEASE": "fixture-lease",
                    "CI_COMMIT_SHA": self.sha,
                    "GH_DEBUG": "api",
                    "GIT_TRACE": "1",
                    "GH_TOKEN": "inherited-fixture",
                },
                clear=True,
            ),
            patch.object(publish, "_cached_token", None),
            patch.object(publish, "_cached_expiration", None),
            patch.object(publish, "_mints", 0),
        ]
        for patcher in patchers:
            patcher.start()
            self.addCleanup(patcher.stop)

    def response(self, expiration=None):
        import io
        import json

        expiry = (
            expiration
            or (
                self.datetime.now(self.timezone.utc) + self.timedelta(hours=1)
            ).isoformat()
        )
        return io.BytesIO(
            json.dumps({"token": "fresh-fixture-only", "expires_at": expiry}).encode()
        )

    def test_fixed_endpoint_bound_identity_and_cached_redemption(self):
        import json

        with patch("urllib.request.build_opener") as build:
            build.return_value.open.return_value = self.response()
            self.assertEqual(publish.credentials(), "fresh-fixture-only")
            self.assertEqual(publish.credentials(), "fresh-fixture-only")
        build.return_value.open.assert_called_once()
        request = build.return_value.open.call_args.args[0]
        self.assertEqual(
            request.full_url, "https://woodpecker.bolens.dev/publisher/credentials"
        )
        self.assertEqual(request.get_header("User-agent"), "woodpecker-publisher/1")
        body = json.loads(request.data)
        self.assertEqual(
            {key: body[key] for key in ["lease", "repository", "commit", "purpose"]},
            {
                "lease": "fixture-lease",
                "repository": publish.REPOSITORY,
                "commit": self.sha,
                "purpose": "github",
            },
        )
        self.assertRegex(body["request_id"], r"^[0-9a-f]{32}$")

    def test_near_expiry_refreshes_and_does_not_reuse_old_token(self):
        publish._cached_token = "old-fixture-only"
        publish._cached_expiration = self.datetime.now(
            self.timezone.utc
        ) + self.timedelta(seconds=60)
        with patch("urllib.request.build_opener") as build:
            build.return_value.open.return_value = self.response()
            self.assertEqual(publish.credentials(), "fresh-fixture-only")
        self.assertEqual(publish._mints, 1)

    def test_expired_broker_response_denied(self):
        with patch("urllib.request.build_opener") as build:
            build.return_value.open.return_value = self.response("2000-01-01T00:00:00Z")
            with self.assertRaises(RuntimeError):
                publish.credentials()
        self.assertIsNone(publish._cached_token)

    def test_mint_budget_fails_before_network(self):
        publish._mints = 8
        with (
            patch("urllib.request.build_opener") as build,
            self.assertRaises(ValueError),
        ):
            publish.credentials()
        build.assert_not_called()

    def test_subprocess_gets_only_fresh_token_not_lease_or_debug(self):
        with patch("urllib.request.build_opener") as build:
            build.return_value.open.return_value = self.response()
            env = publish.publisher_environment()
        self.assertEqual(env["GH_TOKEN"], "fresh-fixture-only")
        self.assertNotIn("PUBLISHER_LEASE", env)
        self.assertNotIn("GH_DEBUG", env)
        self.assertNotIn("GIT_TRACE", env)


class ForgeCoordinatorTests(unittest.TestCase):
    def setUp(self):
        self.sha, self.head, self.base, self.tree = [
            letter * 40 for letter in ["a", "c", "b", "d"]
        ]
        self.tag = "v1.2.3"
        self.env = {
            "GITHUB_ACTIONS": "true",
            "GITHUB_REPOSITORY": publish.REPOSITORY,
            "GITHUB_EVENT_NAME": "workflow_dispatch",
            "GITHUB_REF_TYPE": "tag",
            "GITHUB_REF": "refs/tags/" + self.tag,
            "GITHUB_REF_NAME": self.tag,
            "GITHUB_WORKFLOW_REF": publish.REPOSITORY
            + "/.github/workflows/release.yml@refs/tags/"
            + self.tag,
            "GITHUB_SHA": self.sha,
            "WOODPECKER_PUBLISHING": "true",
            "WOODPECKER_PUBLISHER_ID": "42",
            "GITHUB_COORDINATOR_TOKEN": "platform-fixture-only",
            "PACKAGING_PR": "7",
            "PACKAGING_HEAD": self.head,
            "PACKAGING_BASE": self.base,
            "PACKAGING_TREE": self.tree,
        }
        for patcher in [
            patch.dict(os.environ, self.env, clear=True),
            patch.object(publish, "_coordinator_token", None),
            patch.object(publish, "_coordinator_request", None),
        ]:
            patcher.start()
            self.addCleanup(patcher.stop)
        self.intent = {
            "repository": publish.REPOSITORY,
            "commit": self.sha,
            "tag": self.tag,
            "pr": "7",
            "head": self.head,
            "base": self.base,
            "tree": self.tree,
        }
        repo = {"id": 123, "full_name": publish.REPOSITORY}
        self.pr = {
            "number": 7,
            "state": "open",
            "merged_at": None,
            "user": {"id": 42},
            "head": {
                "repo": dict(repo),
                "ref": "chore/woodpecker-packaging-" + self.tag,
                "sha": self.head,
            },
            "base": {"repo": dict(repo), "ref": "main", "sha": self.base},
        }

    def test_only_exact_enabled_signed_tag_workflow_can_authorize(self):
        with patch.object(publish, "run", side_effect=[self.sha, ""]):
            self.assertEqual(publish.authorize_coordinator(), self.intent)
        self.assertNotIn("GITHUB_COORDINATOR_TOKEN", os.environ)
        self.assertEqual(publish.credentials(), "platform-fixture-only")
        self.assertNotIn("GITHUB_COORDINATOR_TOKEN", publish.publisher_environment())

    def test_pr_fork_branch_wrong_source_or_disabled_context_cannot_authorize(self):
        for key, value in [
            ("GITHUB_EVENT_NAME", "pull_request"),
            ("GITHUB_REPOSITORY", "fork/other"),
            ("GITHUB_REF_TYPE", "branch"),
            ("WOODPECKER_PUBLISHING", "false"),
            (
                "GITHUB_WORKFLOW_REF",
                publish.REPOSITORY + "/.github/workflows/release.yml@refs/heads/main",
            ),
            ("PACKAGING_HEAD", "not-a-sha"),
        ]:
            with (
                self.subTest(key=key),
                patch.dict(os.environ, {key: value}),
                patch.object(publish, "run", side_effect=[self.sha, ""]),
                patch.object(publish, "api") as api,
                self.assertRaises(ValueError),
            ):
                publish.authorize_coordinator()
            api.assert_not_called()
            self.assertIsNone(publish._coordinator_token)

    def test_native_handoff_does_not_hold_runner_waiting_for_checks(self):
        import json
        import tempfile

        with (
            tempfile.TemporaryDirectory() as temporary,
            patch.object(publish, "OUTPUT", Path(temporary)),
            patch.object(
                publish, "api", side_effect=[self.pr, {"sha": self.base}, None]
            ) as api,
            patch.object(publish, "finish_packaging") as wait,
        ):
            publish.request_packaging_merge(
                7,
                self.head,
                self.base,
                self.tree,
                self.pr["head"]["ref"],
                123,
                42,
                self.sha,
                self.tag,
            )
            receipt = json.loads(
                (Path(temporary) / "packaging-receipt.json").read_text()
            )
        self.assertEqual(receipt["status"], "packaging_pending")
        self.assertNotIn("merge_commit", receipt)
        wait.assert_not_called()
        dispatch = api.call_args_list[-1]
        self.assertEqual(
            dispatch.args[:2],
            (
                f"repos/{publish.REPOSITORY}/actions/workflows/release.yml/dispatches",
                "POST",
            ),
        )
        self.assertEqual(dispatch.args[2]["ref"], self.tag)
        self.assertEqual(dispatch.args[2]["inputs"]["packaging_head"], self.head)
        self.assertEqual(dispatch.args[2]["inputs"]["packaging_tree"], self.tree)

    def test_async_intent_cannot_drift_pr_head_base_or_generated_tree(self):
        publish.validate_coordinator_intent(
            self.intent, self.base, self.tree, [self.pr], {"packaging/file"}
        )
        import copy

        scenarios = [(self.base, "e" * 40, [self.pr]), ("e" * 40, self.tree, [self.pr])]
        wrong = copy.deepcopy(self.pr)
        wrong["head"]["sha"] = "e" * 40
        scenarios.append((self.base, self.tree, [wrong]))
        wrong = copy.deepcopy(self.pr)
        wrong["number"] = 8
        scenarios.append((self.base, self.tree, [wrong]))
        for base, tree, prior in scenarios:
            with (
                self.subTest(base=base, tree=tree, prior=prior),
                self.assertRaises(ValueError),
            ):
                publish.validate_coordinator_intent(
                    self.intent, base, tree, prior, {"packaging/file"}
                )

    def test_foreign_release_receipt_cannot_reach_merge_or_pr_creation(self):
        import json
        import tempfile

        with (
            tempfile.TemporaryDirectory() as temporary,
            patch.object(publish, "OUTPUT", Path(temporary)),
            patch.object(publish, "authorize_coordinator", return_value=self.intent),
            patch.object(publish, "verify_tag_source"),
            patch.object(publish, "packaging") as packaging,
        ):

            def download(tag, name, directory):
                (directory / name).write_text(
                    json.dumps(dict(self.intent, commit="f" * 40))
                )

            with (
                patch.object(publish, "download", side_effect=download),
                self.assertRaises(ValueError),
            ):
                publish.finalize_packaging()
            packaging.assert_not_called()
            self.assertEqual(
                json.loads((Path(temporary) / "packaging-receipt.json").read_text())[
                    "status"
                ],
                "failed",
            )


class CoordinatorSignatureTests(unittest.TestCase):
    def test_packaging_hashes_require_tag_specific_signature_and_exact_subjects(self):
        import tempfile

        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / "SHA256SUMS").write_text("a" * 64 + "  archive.tar.gz\n")
            record = {
                "tag": "v1.2.3",
                "assets": {"archive.tar.gz": "a" * 64, "SHA256SUMS": "b" * 64},
            }
            with patch.object(publish, "run") as cosign:
                publish.verify_signed_checksums(directory, record)
            command = cosign.call_args.args
            self.assertIn(
                "https://github.com/bolens/appicon/.github/workflows/release.yml@refs/tags/v1.2.3",
                command,
            )
            self.assertIn("https://token.actions.githubusercontent.com", command)
            record["assets"]["archive.tar.gz"] = "c" * 64
            with patch.object(publish, "run"), self.assertRaises(ValueError):
                publish.verify_signed_checksums(directory, record)


if __name__ == "__main__":
    unittest.main()
