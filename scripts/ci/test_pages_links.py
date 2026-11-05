"""Transient link retries preserve every original checker failure and coverage."""

from contextlib import redirect_stderr, redirect_stdout
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import unittest
from unittest.mock import Mock

SPEC = importlib.util.spec_from_file_location('pages_links', Path(__file__).with_name('check-pages-links.py'))
LINKS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(LINKS)


def report(*codes, unknown=0, timeouts=0, url='https://example.invalid/page'):
    entries = [{'url': url, 'status': {'code': code, 'text': f'Rejected status code: {code} fixture'}}
               for code in codes]
    value = dict.fromkeys(LINKS.COUNTERS, 0)
    value.update(total=max(1, len(codes)), unique=1, successful=0 if codes else 1,
                 errors=len(codes), unknown=unknown, timeouts=timeouts,
                 error_map={'_site/index.html': entries} if entries else {}, timeout_map={})
    return value


def response(*codes, **kwargs):
    return (2 if codes or kwargs.get('unknown') or kwargs.get('timeouts') else 0,
            json.dumps(report(*codes, **kwargs)), '')


class PagesLinkRetries(unittest.TestCase):
    def invoke(self, replies):
        run = Mock(side_effect=replies)
        sleep = Mock()
        output, diagnostics = io.StringIO(), io.StringIO()
        with redirect_stdout(output), redirect_stderr(diagnostics):
            code = LINKS.check_links(run=run, sleep=sleep)
        return code, run, sleep, output.getvalue(), diagnostics.getvalue()

    def test_504_then_success_preserves_exact_coverage_and_original_reports(self):
        first, second = response(504), response()
        code, run, sleep, output, _ = self.invoke([first, second])
        self.assertEqual(code, 0)
        self.assertEqual(run.call_count, 2)
        self.assertEqual(run.call_args.args[0], [
            'lychee', '--no-progress', '--accept', '200,429', '--exclude',
            '^https://bolens[.]github[.]io/appicon/changelog/$', '--format', 'json', '_site/**/*.html'])
        self.assertEqual(run.call_args_list[0], run.call_args_list[1])
        sleep.assert_called_once_with(2)
        self.assertIn(first[1], output)
        self.assertIn(second[1], output)

    def test_persistent_5xx_fails_after_three_complete_scans(self):
        code, run, sleep, _, _ = self.invoke([response(504), response(503), response(502)])
        self.assertEqual(code, 2)
        self.assertEqual(run.call_count, 3)
        self.assertEqual([call.args[0] for call in sleep.call_args_list], [2, 4])

    def test_mixed_404_nonhttp_unknown_and_timeout_never_retry(self):
        replies = [response(504, 404), response(504, url='file:///missing'),
                   response(504, unknown=1), response(504, timeouts=1)]
        for reply in replies:
            with self.subTest(reply=reply):
                code, run, sleep, _, _ = self.invoke([reply])
                self.assertEqual(code, 2)
                self.assertEqual(run.call_count, 1)
                sleep.assert_not_called()

    def test_invalid_json_typed_counts_and_inconsistent_maps_fail_closed(self):
        values = ['not json', json.dumps({}), json.dumps(dict(report(504), errors=True)),
                  json.dumps(dict(report(504), errors=2)), json.dumps(dict(report(504), error_map={}))]
        for value in values:
            run, sleep = Mock(return_value=(2, value, '')), Mock()
            with self.subTest(value=value), redirect_stdout(io.StringIO()), self.assertRaises(ValueError):
                LINKS.check_links(run=run, sleep=sleep)
            self.assertEqual(run.call_count, 1)
            sleep.assert_not_called()

    def test_unknown_status_kinds_cannot_be_reclassified_as_transient(self):
        value = report(504)
        value['error_map']['_site/index.html'][0]['status']['text'] = 'unknown failure'
        code, run, sleep, _, _ = self.invoke([(2, json.dumps(value), '')])
        self.assertEqual(code, 2)
        self.assertEqual(run.call_count, 1)
        sleep.assert_not_called()

    def test_success_exit_cannot_hide_reported_errors(self):
        with redirect_stdout(io.StringIO()), self.assertRaises(ValueError):
            LINKS.check_links(run=Mock(return_value=(0, json.dumps(report(504)), '')))

    def test_process_input_failure_is_preserved_without_retry(self):
        code, run, sleep, _, diagnostics = self.invoke([(1, '', 'invalid input\n')])
        self.assertEqual(code, 1)
        self.assertEqual(run.call_count, 1)
        sleep.assert_not_called()
        self.assertEqual(diagnostics, 'invalid input\n')

    def test_actual_child_timeout_and_output_bound_are_enforced(self):
        with self.assertRaises(TimeoutError):
            LINKS.read_result([sys.executable, '-c', 'import time; time.sleep(10)'], timeout=0.2)
        with self.assertRaises(ValueError):
            LINKS.read_result([sys.executable, '-c', 'print("x" * 5000)'], max_bytes=1024)
        code, output, diagnostics = LINKS.read_result(
            [sys.executable, '-c', 'import sys; print("out"); print("err", file=sys.stderr)'])
        self.assertEqual((code, output, diagnostics), (0, 'out\n', 'err\n'))

    def test_counter_is_occurrences_not_unique_urls(self):
        value = report(*([504] * 9))
        value['unique'] = 2
        self.assertEqual(LINKS.classify(value), (False, True))

    def test_subprocess_timeout_is_not_a_retryable_http_response(self):
        run, sleep = Mock(side_effect=subprocess.TimeoutExpired('lychee', 180)), Mock()
        with self.assertRaises(subprocess.TimeoutExpired):
            LINKS.check_links(run=run, sleep=sleep)
        self.assertEqual(run.call_count, 1)
        sleep.assert_not_called()
