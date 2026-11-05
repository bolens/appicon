"""Release helpers use isolated local Git/signing fixtures, never product tags."""

import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
GIT = shutil.which('git')
SPEC = importlib.util.spec_from_file_location('release_publisher', ROOT / 'scripts/ci/woodpecker-publish.py')
PUBLISH = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PUBLISH)


class SignedReleaseHelper(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.scratch = Path(self.temporary.name)
        self.repo = self.scratch / 'checkout'
        self.repo.mkdir()
        self.remote = self.scratch / 'remote.git'
        self.env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
        self.env.pop('SSH_AUTH_SOCK', None)
        self.env.update(HOME=str(self.scratch), XDG_CONFIG_HOME=str(self.scratch / 'xdg'),
                        GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
                        GIT_TERMINAL_PROMPT='0', GIT_ALLOW_PROTOCOL='file')
        self.git('init', '--bare', str(self.remote))
        self.git('init', '-b', 'main')
        for key, value in [('user.name', 'Release fixture'), ('user.email', 'fixture@example.invalid'),
                           ('core.hooksPath', str(self.scratch / 'empty-hooks')),
                           ('commit.gpgSign', 'false'), ('protocol.file.allow', 'always')]:
            self.git('config', key, value)
        (self.repo / 'scripts/ci').mkdir(parents=True)
        shutil.copyfile(ROOT / 'scripts/ci/cut-release.sh', self.repo / 'scripts/ci/cut-release.sh')
        (self.repo / 'VERSION').write_text('1.2.3\n')
        (self.repo / 'Makefile').write_text('check:\n\t@true\n')
        self.git('add', '.')
        self.git('commit', '-m', 'fixture: prepared source')
        self.sha = self.git('rev-parse', 'HEAD')
        self.git('remote', 'add', 'origin', 'https://github.com/bolens/appicon.git')
        self.git('config', 'url.' + self.remote.as_uri() + '.insteadOf',
                 'https://github.com/bolens/appicon.git')
        self.assertEqual(self.git('remote', 'get-url', 'origin'), self.remote.as_uri())
        self.git('push', 'origin', 'main')
        key = self.scratch / 'signing-key'
        subprocess.run(['ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', str(key)],
                       env=self.env, check=True, capture_output=True)
        allowed = self.scratch / 'allowed-signers'
        allowed.write_text('fixture@example.invalid ' + key.with_suffix('.pub').read_text())
        self.git('config', 'gpg.format', 'ssh')
        self.git('config', 'user.signingKey', str(key))
        self.git('config', 'gpg.ssh.allowedSignersFile', str(allowed))
        # Only helper subprocesses use this recorder; any attempted push fails.
        bindir = self.scratch / 'bin'
        bindir.mkdir()
        self.log = self.scratch / 'git-calls'
        wrapper = bindir / 'git'
        wrapper.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$FIXTURE_GIT_LOG"\n'
                           'if [ "$1" = push ]; then exit 98; fi\nexec "$FIXTURE_REAL_GIT" "$@"\n')
        wrapper.chmod(0o755)
        self.run_env = dict(self.env, PATH=str(bindir) + os.pathsep + self.env['PATH'],
                            FIXTURE_GIT_LOG=str(self.log), FIXTURE_REAL_GIT=GIT)

    def git(self, *args, cwd=None):
        return subprocess.check_output([GIT, *args], cwd=cwd or self.repo, env=self.env,
                                       stderr=subprocess.DEVNULL, text=True).strip()

    def invoke(self, *args):
        result = subprocess.run(['bash', 'scripts/ci/cut-release.sh', *(args or ('v1.2.3', self.sha))],
                                cwd=self.repo, env=self.run_env, capture_output=True, text=True,
                                timeout=30)
        calls = self.log.read_text() if self.log.exists() else ''
        self.assertFalse(any(line.startswith('push ') for line in calls.splitlines()))
        return result

    def assert_denied(self, *args):
        result = self.invoke(*args)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.git('tag', '--list'), '')
        return result

    def test_creates_signed_annotated_exact_object_and_prints_only_tag_push(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.git('cat-file', '-t', 'refs/tags/v1.2.3'), 'tag')
        self.assertEqual(self.git('rev-parse', 'v1.2.3^{commit}'), self.sha)
        self.git('verify-tag', 'v1.2.3')
        self.assertIn('git push origin refs/tags/v1.2.3:refs/tags/v1.2.3', result.stdout)
        self.assertNotIn('git push origin HEAD', result.stdout)
        self.assertNotIn('git push origin main', result.stdout)
        self.assertEqual(self.git('--git-dir', str(self.remote), 'tag', '--list'), '')

    def test_arguments_and_source_sha_are_explicit_and_stable(self):
        for args in [('v1.2.3',), ('1.2.3', self.sha), ('v1.2.3-rc1', self.sha),
                     ('v1.2.3', self.sha[:12]), ('v1.2.3', 'a' * 40)]:
            with self.subTest(args=args):
                self.assert_denied(*args)

    def test_offmain_dirty_and_noncanonical_origin_are_denied(self):
        self.git('checkout', '-b', 'release/fixture')
        self.assert_denied()
        self.git('checkout', 'main')
        dirty = self.repo / 'untracked'
        dirty.touch()
        self.assert_denied()
        dirty.unlink()
        self.git('remote', 'set-url', 'origin', str(self.remote))
        self.assert_denied()

    def test_missing_malformed_and_mismatched_prepared_version_are_denied(self):
        for value in [None, '', '1.2', '1.2.3-rc1\n', '1.2.3\n\n', '1.2.4\n']:
            path = self.repo / 'VERSION'
            if value is None:
                path.unlink(missing_ok=True)
            else:
                path.write_text(value)
            self.git('add', '-A')
            self.git('commit', '-m', 'fixture: invalid prepared version')
            self.sha = self.git('rev-parse', 'HEAD')
            self.git('push', 'origin', 'main')
            with self.subTest(value=value):
                self.assert_denied()

    def test_existing_local_and_remote_tags_are_not_replaced(self):
        self.git('tag', 'v1.2.3')
        old = self.git('rev-parse', 'refs/tags/v1.2.3')
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.git('rev-parse', 'refs/tags/v1.2.3'), old)
        self.git('push', 'origin', 'refs/tags/v1.2.3')
        self.git('tag', '-d', 'v1.2.3')
        self.assert_denied()
        self.assertEqual(self.git('--git-dir', str(self.remote), 'rev-parse', 'refs/tags/v1.2.3'), old)

    def test_remote_main_advancement_is_rejected(self):
        other = self.scratch / 'other'
        self.git('clone', '-b', 'main', str(self.remote), str(other))
        self.git('config', 'user.name', 'Other fixture', cwd=other)
        self.git('config', 'user.email', 'other@example.invalid', cwd=other)
        (other / 'new-main').touch()
        self.git('add', '.', cwd=other)
        self.git('commit', '-m', 'fixture: main advanced', cwd=other)
        self.git('push', 'origin', 'main', cwd=other)
        result = self.assert_denied()
        self.assertIn('origin/main has advanced', result.stderr)

    def commit_fixture(self):
        self.git('add', '.')
        self.git('commit', '-m', 'fixture: check behavior')
        self.sha = self.git('rev-parse', 'HEAD')
        self.git('push', 'origin', 'main')

    def test_fetch_and_check_failures_never_create_a_tag(self):
        self.git('config', '--unset', 'url.' + self.remote.as_uri() + '.insteadOf')
        missing = self.scratch / 'missing.git'
        self.git('config', 'url.' + missing.as_uri() + '.insteadOf',
                 'https://github.com/bolens/appicon.git')
        self.assert_denied()
        self.git('config', '--unset', 'url.' + missing.as_uri() + '.insteadOf')
        self.git('config', 'url.' + self.remote.as_uri() + '.insteadOf',
                 'https://github.com/bolens/appicon.git')
        (self.repo / 'Makefile').write_text('check:\n\t@false\n')
        self.commit_fixture()
        self.assert_denied()

    def test_main_advancement_during_checks_is_rechecked(self):
        import shlex
        git = shlex.quote(GIT)
        remote = shlex.quote(str(self.remote))
        command = (f'{git} --git-dir {remote} update-ref refs/heads/main '
                   f'$$({git} --git-dir {remote} rev-parse refs/heads/future-fixture)')
        (self.repo / 'Makefile').write_text('check:\n\t@' + command + '\n')
        self.commit_fixture()
        self.git('commit', '--allow-empty', '-m', 'fixture: later main')
        future = self.git('rev-parse', 'HEAD')
        self.git('push', 'origin', 'HEAD:refs/heads/future-fixture')
        self.git('update-ref', 'refs/heads/main', self.sha)
        result = self.assert_denied()
        self.assertIn('origin/main has advanced', result.stderr)
        self.assertEqual(self.git('--git-dir', str(self.remote), 'rev-parse', 'main'), future)

    def test_signing_and_verification_failures_never_push(self):
        self.git('config', 'user.signingKey', str(self.scratch / 'absent-key'))
        self.assert_denied()
        self.git('config', 'user.signingKey', str(self.scratch / 'signing-key'))
        self.git('config', 'gpg.ssh.allowedSignersFile', str(self.scratch / 'absent-signers'))
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git('cat-file', '-t', 'refs/tags/v1.2.3'), 'tag')
        self.assertNotIn('Push only', result.stdout)


class PreparedReleaseVersion(unittest.TestCase):
    def test_exact_stable_version_is_required_without_packaging_fallback(self):
        for value in ['', '1.2', 'v1.2.3', '1.2.3-rc1', '1.2.3\n\n', ' 1.2.3\n']:
            with patch.object(PUBLISH.Path, 'read_text', return_value=value), self.subTest(value=value), \
                    self.assertRaises(ValueError):
                PUBLISH.prepared_version()
        with patch.object(PUBLISH.Path, 'read_text', side_effect=FileNotFoundError), \
                self.assertRaises(FileNotFoundError):
            PUBLISH.prepared_version()

    def test_new_source_version_ignores_published_flake_and_keeps_signed_source_checks(self):
        sha = 'a' * 40
        replies = [{'object': {'type': 'tag', 'sha': 'b' * 40}},
                   {'verification': {'verified': True}, 'object': {'type': 'commit', 'sha': sha}},
                   {'status': 'identical'}]
        def source(path, *args, **kwargs):
            self.assertEqual(path, Path('VERSION'))
            return '1.2.3\n'
        with patch.object(PUBLISH.Path, 'read_text', autospec=True, side_effect=source), \
                patch.object(PUBLISH, 'api', side_effect=replies):
            PUBLISH.verify_tag_source(sha, 'v1.2.3')
        with patch.object(PUBLISH, 'prepared_version', return_value='1.2.4'), \
                patch.object(PUBLISH, 'api', side_effect=replies), self.assertRaises(ValueError):
            PUBLISH.verify_tag_source(sha, 'v1.2.3')

    def test_packaging_checks_new_main_version_before_generator(self):
        sha = 'a' * 40
        record = {'commit': sha, 'tag': 'v1.2.3', 'assets': {
            'appicon_v1.2.3_linux_amd64.tar.gz': '1' * 64,
            'appicon_v1.2.3_linux_arm64.tar.gz': '2' * 64}}
        class ReachedGenerator(RuntimeError):
            pass
        for current in ['1.2.3', '1.2.4']:
            calls = []
            def run(*args, **kwargs):
                calls.append(args)
                if args[:2] == ('python3', 'scripts/ci/update-release-packaging.py'):
                    raise ReachedGenerator
                return 'c' * 40
            with patch.object(PUBLISH, 'guard', return_value=(sha, 'v1.2.3')), \
                    patch.object(PUBLISH.Path, 'read_text', return_value=json.dumps(record)), \
                    patch.object(PUBLISH, 'verify_assets'), \
                    patch.object(PUBLISH, 'api', side_effect=[{'id': 1}, []]), \
                    patch.object(PUBLISH, 'approved_publisher_id', return_value=42), \
                    patch('urllib.request.urlopen', return_value=io.BytesIO(b'fixture source archive')), \
                    patch.object(PUBLISH, 'prepared_version', return_value=current), \
                    patch.object(PUBLISH, 'run', side_effect=run):
                with self.assertRaises(ReachedGenerator if current == '1.2.3' else ValueError):
                    PUBLISH.packaging()
            self.assertIn(('git', 'checkout', '--detach', 'c' * 40), calls)
            self.assertEqual(any(c[0] == 'python3' for c in calls), current == '1.2.3')

    def test_postrelease_generator_leaves_prepared_version_and_aligns_published_group(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ['flake.nix', 'nix', 'packaging/aur', 'scripts/ci']:
                src = ROOT / name
                if src.is_dir():
                    shutil.copytree(src, root / name, dirs_exist_ok=True)
                else:
                    shutil.copyfile(src, root / name)
            (root / 'VERSION').write_text('1.2.3\n')
            (root / 'CHANGELOG.md').write_text('## [1.2.3] - 2026-11-05\n')
            subprocess.run(['python3', 'scripts/ci/update-release-packaging.py', '1.2.3',
                            '1' * 64, '2' * 64, '3' * 64], cwd=root, check=True, capture_output=True)
            self.assertEqual((root / 'VERSION').read_text(), '1.2.3\n')
            subprocess.run(['bash', 'scripts/ci/check-packaging-versions.sh'], cwd=root,
                           check=True, capture_output=True)
