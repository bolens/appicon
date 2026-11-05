"""Public release identity, freshness and bounded transient failure behavior."""

import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch, MagicMock
import urllib.error

spec = importlib.util.spec_from_file_location(
    "metadata", Path(__file__).with_name("public-release-metadata.py")
)
metadata = importlib.util.module_from_spec(spec)
spec.loader.exec_module(metadata)


class PublicReleaseTests(unittest.TestCase):
    def response(self):
        response = MagicMock()
        response.__enter__.return_value.geturl.return_value = (
            "https://github.com/" + metadata.REPOSITORY + "/releases/tag/v1.2.3"
        )
        return response

    def test_canonical_stable_release_only(self):
        self.assertEqual(
            metadata.parse_release_url(
                "https://github.com/" + metadata.REPOSITORY + "/releases/tag/v1.2.3"
            ),
            "v1.2.3",
        )
        for url in [
            "https://attacker.example/release/v1.2.3",
            "http://github.com/" + metadata.REPOSITORY + "/releases/tag/v1.2.3",
            "https://github.com/other/repo/releases/tag/v1.2.3",
            "https://github.com/" + metadata.REPOSITORY + "/releases/tag/v1.2.3-rc1",
        ]:
            with self.subTest(url=url), self.assertRaises(ValueError):
                metadata.parse_release_url(url)

    def test_uses_public_redirect_not_rest_or_credentials(self):
        with patch("urllib.request.urlopen", return_value=self.response()) as fetch:
            record = metadata.latest()
        request = fetch.call_args.args[0]
        self.assertEqual(
            request.full_url,
            "https://github.com/" + metadata.REPOSITORY + "/releases/latest",
        )
        self.assertEqual(request.method, "HEAD")
        self.assertNotIn("Authorization", request.headers)
        self.assertEqual(record["tag"], "v1.2.3")

    def test_transient_errors_retry_bounded_without_stale_fallback(self):
        error = urllib.error.HTTPError(
            "https://github.com/example", 429, "rate limited", {}, None
        )
        with (
            patch(
                "urllib.request.urlopen", side_effect=[error, self.response()]
            ) as fetch,
            patch.object(metadata.time, "sleep") as sleep,
        ):
            self.assertEqual(metadata.latest()["tag"], "v1.2.3")
        self.assertEqual(fetch.call_count, 2)
        sleep.assert_called_once_with(1)
        with (
            patch("urllib.request.urlopen", side_effect=error) as fetch,
            patch.object(metadata.time, "sleep"),
            self.assertRaises(urllib.error.HTTPError),
        ):
            metadata.latest()
        self.assertEqual(fetch.call_count, 3)


if __name__ == "__main__":
    unittest.main()
