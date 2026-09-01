#!/usr/bin/env python3
"""Validate the static GitHub Pages user guide."""

from __future__ import annotations

import argparse
import json
import sys
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlparse

REQUIRED_ROUTES = (
    "index.html",
    "install/index.html",
    "guide/index.html",
    "help/index.html",
    "search/index.html",
    "architecture/index.html",
)


class PageParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.h1_count = 0
        self.main_count = 0
        self.title_count = 0
        self.links: list[str] = []
        self.meta: dict[str, str] = {}
        self.scripts: list[str] = []
        self.styles: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        values = dict(attrs)
        if tag == "h1":
            self.h1_count += 1
        elif tag == "main":
            self.main_count += 1
        elif tag == "title":
            self.title_count += 1
        elif tag == "a" and values.get("href"):
            self.links.append(values["href"] or "")
        elif tag == "meta":
            key = values.get("name") or values.get("property")
            if key and values.get("content"):
                self.meta[key] = values["content"] or ""
        elif tag == "script" and values.get("src"):
            self.scripts.append(values["src"] or "")
        elif tag == "link" and values.get("href"):
            self.styles.append(values["href"] or "")


def local_target(page: Path, site: Path, href: str) -> Path | None:
    parsed = urlparse(href)
    if parsed.scheme or parsed.netloc or href.startswith(("#", "mailto:")):
        return None
    path = unquote(parsed.path)
    if not path:
        return None
    target = site / path.lstrip("/") if path.startswith("/") else page.parent / path
    if path.endswith("/"):
        target /= "index.html"
    elif not target.suffix:
        target /= "index.html"
    return target.resolve()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--site", type=Path, default=Path("site"))
    parser.add_argument("--architecture", type=Path, required=True)
    args = parser.parse_args()

    site = args.site.resolve()
    errors: list[str] = []
    for route in REQUIRED_ROUTES:
        if not (site / route).is_file():
            errors.append(f"missing route: {route}")
    for asset in (
        "site.webmanifest",
        "assets/favicon.png",
        "assets/apple-touch-icon.png",
        "assets/icon-192.png",
        "assets/icon-512.png",
        "assets/social-card.png",
    ):
        if not (site / asset).is_file():
            errors.append(f"missing discovery asset: {asset}")

    pages = sorted(site.rglob("*.html"))
    for page in pages:
        parsed = PageParser()
        parsed.feed(page.read_text(encoding="utf-8"))
        rel = page.relative_to(site)
        if parsed.h1_count != 1:
            errors.append(f"{rel}: expected one h1, found {parsed.h1_count}")
        if parsed.main_count != 1:
            errors.append(f"{rel}: expected one main, found {parsed.main_count}")
        if parsed.title_count != 1:
            errors.append(f"{rel}: expected one title, found {parsed.title_count}")
        for key in ("description", "og:title", "og:description", "og:image"):
            if key not in parsed.meta:
                errors.append(f"{rel}: missing {key} metadata")
        if not any("favicon" in href for href in parsed.styles):
            errors.append(f"{rel}: missing favicon")
        if not any(src.endswith("site.js") for src in parsed.scripts):
            errors.append(f"{rel}: missing shared site.js")
        for href in parsed.links:
            if "docs/architecture/" in href and args.architecture.is_file():
                continue
            target = local_target(page, site, href)
            if target is not None and not target.exists():
                errors.append(f"{rel}: broken internal link {href}")

    home = (site / "index.html").read_text(encoding="utf-8")
    for contract in ("rel=\"canonical\"", "og:type", "og:url", "og:site_name", "og:image:width",
                     "twitter:title", "twitter:description", "twitter:image", "twitter:image:alt",
                     "rel=\"apple-touch-icon\"", "rel=\"manifest\""):
        if contract not in home:
            errors.append(f"index.html: missing discovery contract {contract}")

    index = site / "search-index.json"
    if not index.is_file():
        errors.append("missing search-index.json")
    else:
        try:
            records = json.loads(index.read_text(encoding="utf-8"))
            if len(records) < 5:
                errors.append("search-index.json: expected at least five records")
            for record in records:
                target = local_target(site / "index.html", site, record["url"])
                if target is not None and not target.exists():
                    errors.append(f"search-index.json: broken URL {record['url']}")
        except (json.JSONDecodeError, KeyError, TypeError) as exc:
            errors.append(f"search-index.json: {exc}")

    combined_html = "\n".join(page.read_text(encoding="utf-8") for page in pages)
    if "__SITE_VERSION__" not in combined_html:
        errors.append("missing __SITE_VERSION__ release token")
    if "issues/new?title=" not in combined_html:
        errors.append("missing prefilled feedback link")

    script = site / "assets/site.js"
    if not script.is_file():
        errors.append("missing assets/site.js")
    else:
        source = script.read_text(encoding="utf-8")
        for behavior in ("navigator.clipboard", "aria-pressed", "aria-live", "search-index.json"):
            if behavior not in source:
                errors.append(f"assets/site.js: missing {behavior} behavior")

    css = site / "assets/site.css"
    if not css.is_file():
        errors.append("missing assets/site.css")
    elif "forced-colors: active" not in css.read_text(encoding="utf-8"):
        errors.append("assets/site.css: missing forced-colors support")

    if not args.architecture.is_file():
        errors.append(f"missing architecture artifact: {args.architecture}")

    if errors:
        print("\n".join(f"ERROR: {error}" for error in errors), file=sys.stderr)
        return 1
    print(f"pages-site: OK ({len(pages)} pages)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
