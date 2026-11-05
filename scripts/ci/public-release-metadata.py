#!/usr/bin/env python3
"""Resolve the canonical public stable release without the shared REST API quota."""

import json
import re
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import unquote, urlparse
import urllib.error
import urllib.request

REPOSITORY = "bolens/appicon"


def parse_release_url(url):
    parsed = urlparse(url)
    prefix = "/" + REPOSITORY + "/releases/tag/"
    if (
        parsed.scheme != "https"
        or parsed.netloc != "github.com"
        or not parsed.path.startswith(prefix)
        or parsed.query
        or parsed.fragment
    ):
        raise ValueError(
            "latest release redirect did not identify the canonical GitHub release"
        )
    tag = unquote(parsed.path[len(prefix) :])
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        raise ValueError("latest release is not a supported stable version")
    return tag


def latest():
    request = urllib.request.Request(
        "https://github.com/" + REPOSITORY + "/releases/latest",
        method="HEAD",
        headers={
            "User-Agent": "Woodpecker-CI-public-release-metadata",
            "Cache-Control": "no-cache",
        },
    )
    for attempt in range(3):
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                url = response.geturl()
            return {
                "repository": REPOSITORY,
                "tag": parse_release_url(url),
                "url": url,
                "checked_at": datetime.now(timezone.utc).isoformat(),
            }
        except urllib.error.HTTPError as error:
            if error.code not in {403, 429, 500, 502, 503, 504} or attempt == 2:
                raise
        except urllib.error.URLError:
            if attempt == 2:
                raise
        time.sleep(2**attempt)
    raise RuntimeError("public release metadata could not be refreshed")


if __name__ == "__main__":
    Path(sys.argv[1]).write_text(json.dumps(latest(), indent=2) + "\n")
