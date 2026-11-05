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
            publish.finish_packaging("https://github.com/example/pr/1", self.sha)
        run.assert_not_called()

    def test_passed_checks_use_exact_head_protected_squash(self):
        import subprocess

        result = subprocess.CompletedProcess(
            [], 0, '[{"bucket":"pass","name":"native"}]', ""
        )
        with (
            patch.object(subprocess, "run", return_value=result),
            patch.object(
                publish,
                "run",
                side_effect=["", '{"state":"MERGED","mergeCommit":{"oid":"a"}}'],
            ) as run,
        ):
            publish.finish_packaging("https://github.com/example/pr/1", self.sha)
        command = run.call_args_list[0].args
        self.assertIn("--squash", command)
        self.assertNotIn("--admin", command)
        self.assertEqual(command[-2:], ("--match-head-commit", self.sha))

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

            with (
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


if __name__ == "__main__":
    unittest.main()
