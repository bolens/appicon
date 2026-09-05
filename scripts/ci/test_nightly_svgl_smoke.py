"""Offline coverage for the live smoke harness, using a fake CLI."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('nightly-svgl-smoke.sh')


class NightlySmokeTests(unittest.TestCase):
    def run_smoke(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cli = root / 'appicon'
            cli.write_text('''#!/usr/bin/env python3
import json, os, pathlib, sys
if sys.argv[1] == 'version':
    print('test')
    sys.exit(0)
root = pathlib.Path(os.environ['SMOKE_TEST_ROOT'])
log = root / 'calls.jsonl'
previous = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
query = sys.argv[-1]
cache = pathlib.Path(os.environ['XDG_CACHE_HOME'])
with log.open('a') as handle:
    handle.write(json.dumps({'query': query, 'cache': str(cache)}) + '\\n')
mode = os.environ['SMOKE_TEST_MODE']
attempt = sum(call['query'] == query for call in previous) + 1
if mode == 'transient' and attempt == 1 or mode == 'persistent':
    print('connection reset by peer', file=sys.stderr)
    sys.exit(2)
if mode == 'miss':
    sys.exit(1)
if mode == 'malformed':
    print('invalid JSON')
    sys.exit(0)
cache.mkdir(parents=True, exist_ok=True)
icon = cache / (query + '.svg')
icon.write_text('<svg xmlns="http://www.w3.org/2000/svg"/>')
print(json.dumps({'path': str(icon), 'source': 'svgl', 'format': 'svg',
                  'cached': mode == 'cached', 'error': None}))
''')
            cli.chmod(0o755)
            sleep = root / 'sleep'
            sleep.write_text('#!/bin/sh\nexit 0\n')
            sleep.chmod(0o755)
            personal_cache = root / 'personal-cache'
            personal_cache.mkdir()
            sentinel = personal_cache / 'keep'
            sentinel.write_text('preserve')
            env = dict(os.environ, APPICON_BIN=str(cli), XDG_CACHE_HOME=str(personal_cache),
                       SMOKE_TEST_ROOT=str(root), SMOKE_TEST_MODE=mode,
                       PATH=f'{root}:{os.environ["PATH"]}')
            result = subprocess.run(['bash', str(SCRIPT)], env=env, text=True,
                                    capture_output=True, timeout=10)
            calls = [json.loads(line) for line in (root / 'calls.jsonl').read_text().splitlines()]
            self.assertEqual(sentinel.read_text(), 'preserve')
            for call in calls:
                self.assertNotEqual(call['cache'], str(personal_cache))
                self.assertFalse(Path(call['cache']).exists(), 'smoke cache was not cleaned')
            return result, calls

    def test_success_uses_and_cleans_isolated_cache(self):
        result, calls = self.run_smoke('success')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([call['query'] for call in calls], ['firefox', 'discord'])

    def test_transient_errors_are_retried(self):
        result, calls = self.run_smoke('transient')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 4)

    def test_persistent_errors_fail_after_three_attempts(self):
        result, calls = self.run_smoke('persistent')
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(len(calls), 3)

    def test_miss_is_not_retried(self):
        result, calls = self.run_smoke('miss')
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertEqual(len(calls), 1)

    def test_invalid_json_and_warm_cache_fail_without_retries(self):
        for mode in ('malformed', 'cached'):
            with self.subTest(mode=mode):
                result, calls = self.run_smoke(mode)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(len(calls), 1)


if __name__ == '__main__':
    unittest.main()
