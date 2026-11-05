#!/usr/bin/env python3
"""Keep Pages link coverage strict while retrying only reported HTTP 5xx failures."""

import json
import os
import selectors
import subprocess
import sys
import time
from urllib.parse import urlsplit

COMMAND = [
    'lychee', '--no-progress', '--accept', '200,429',
    '--exclude', '^https://bolens[.]github[.]io/appicon/changelog/$',
    '--format', 'json', '_site/**/*.html',
]
COUNTERS = ('total', 'unique', 'successful', 'unknown', 'unsupported', 'timeouts',
            'redirects', 'remaps', 'excludes', 'errors', 'cached')


def read_result(command, *, timeout=180, max_bytes=8 * 1024 * 1024):
    """Drain both pipes with a shared byte/time bound; kill only our owned child."""
    deadline = time.monotonic() + timeout
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    buffers = {'stdout': bytearray(), 'stderr': bytearray()}
    try:
        with selectors.DefaultSelector() as reader:
            reader.register(process.stdout, selectors.EVENT_READ, 'stdout')
            reader.register(process.stderr, selectors.EVENT_READ, 'stderr')
            while reader.get_map():
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise TimeoutError('link check exceeded its 180-second attempt bound')
                for key, _ in reader.select(min(remaining, 0.2)):
                    data = os.read(key.fd, 65536)
                    if not data:
                        reader.unregister(key.fileobj)
                        continue
                    buffers[key.data].extend(data)
                    if sum(len(value) for value in buffers.values()) > max_bytes:
                        raise ValueError('link-check output exceeds its 8 MiB bound')
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError('link check exceeded its attempt bound')
            code = process.wait(timeout=remaining)
        return (code, buffers['stdout'].decode('utf-8'),
                buffers['stderr'].decode('utf-8', errors='replace'))
    finally:
        if process.poll() is None:
            process.kill()
        process.wait(timeout=5)
        process.stdout.close()
        process.stderr.close()


def classify(report):
    """Validate the pinned formatter and return (clean, exclusively HTTP 5xx)."""
    if not isinstance(report, dict) or any(
            type(report.get(key)) is not int or report[key] < 0 for key in COUNTERS):
        raise ValueError('invalid lychee report counters')
    errors = report.get('error_map')
    timeouts = report.get('timeout_map')
    if not isinstance(errors, dict) or not isinstance(timeouts, dict):
        raise ValueError('invalid lychee report maps')
    entries = []
    for source, values in errors.items():
        if not isinstance(source, str) or not isinstance(values, list) or not values:
            raise ValueError('invalid lychee error entries')
        entries.extend(values)
    # Pinned 0.24.2 counts occurrences, not unique URLs (proved with 7 HTML inputs).
    if report['errors'] != len(entries) or report['unique'] > report['total']:
        raise ValueError('inconsistent lychee error counts')
    clean = not entries and not timeouts and not any(
        report[key] for key in ('errors', 'timeouts', 'unknown'))
    if clean:
        return True, False
    if not entries or timeouts or report['timeouts'] or report['unknown']:
        return False, False
    for entry in entries:
        if not isinstance(entry, dict) or not isinstance(entry.get('url'), str):
            return False, False
        status = entry.get('status')
        if not isinstance(status, dict):
            return False, False
        code = status.get('code')
        text = status.get('text')
        if (type(code) is not int or not 500 <= code <= 599
                or not isinstance(text, str)
                or not text.startswith(f'Rejected status code: {code} ')):
            return False, False
        uri = urlsplit(entry['url'])
        if (uri.scheme not in ('http', 'https') or not uri.hostname
                or uri.username is not None or uri.password is not None):
            return False, False
    return False, True


def check_links(*, run=read_result, sleep=time.sleep):
    for attempt in range(1, 4):
        code, output, diagnostics = run(COMMAND)
        if output:
            print(output, end='' if output.endswith('\n') else '\n', flush=True)
        if diagnostics:
            print(diagnostics, file=sys.stderr,
                  end='' if diagnostics.endswith('\n') else '\n', flush=True)
        if code not in (0, 2):
            return code if code > 0 else 1
        clean, transient = classify(json.loads(output))
        if code == 0:
            if not clean:
                raise ValueError('successful lychee exit has a failing report')
            return 0
        if clean:
            raise ValueError('failing lychee exit has no reported failures')
        if not transient or attempt == 3:
            return code
        delay = 2 ** attempt
        print(f'lychee attempt {attempt}/3: only HTTP 5xx failures; retrying in {delay}s',
              file=sys.stderr, flush=True)
        sleep(delay)
    raise RuntimeError('unreachable link-check retry state')


def main():
    try:
        return check_links()
    except (OSError, ValueError, TimeoutError, subprocess.TimeoutExpired) as error:
        print(f'Pages link check failed: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
