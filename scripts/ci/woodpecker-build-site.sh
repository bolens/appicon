#!/usr/bin/env bash
# Build in a disposable copy so validation never mutates the release source.
set -euo pipefail
mkdir -p .woodpecker-out
mkdir -p .woodpecker-out/site-tools
curl --fail --location --retry 3 --output .woodpecker-out/lychee.tar.gz https://github.com/lycheeverse/lychee/releases/download/lychee-v0.24.2/lychee-x86_64-unknown-linux-gnu.tar.gz
printf '%s  %s\n' 1f4e0ef7f6554a6ed33dd7ac144fb2e1bbed98598e7af973042fc5cd43951c9a .woodpecker-out/lychee.tar.gz | sha256sum --check --strict
tar -xzf .woodpecker-out/lychee.tar.gz -C .woodpecker-out/site-tools lychee
export PATH="$PWD/.woodpecker-out/site-tools:$PATH"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
git archive HEAD | tar -x -C "$work"
python3 - "$work" <<'LATEST'
import json, sys, urllib.request
from pathlib import Path
with urllib.request.urlopen('https://api.github.com/repos/bolens/appicon/releases/latest', timeout=30) as response:
    release = json.load(response)
(Path(sys.argv[1]) / 'latest-release.json').write_text(json.dumps(release))
LATEST
(cd "$work" && version=$(python3 -c 'import json; print(json.load(open("latest-release.json"))["tag_name"])')
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
python3 scripts/render-changelog.py --name AppIcon --base-url https://bolens.github.io/appicon/ --accent "#5eead4" --social-image assets/social-card.png --favicon assets/favicon.png --docs-shell --output site/changelog/index.html
python3 scripts/ci/check-pages-site.py --site site --architecture docs/architecture/appicon.html --architecture-source docs/architecture/appicon.architecture.json
python3 scripts/ci/build-pages-site.py --architecture docs/architecture/appicon.html --architecture-name appicon.html --version "$version"
python3 -m http.server 4173 --bind 127.0.0.1 --directory _site > /tmp/appicon-pages-http.log 2>&1 &
server=$!
trap 'kill "$server" 2>/dev/null || true' EXIT
npx --yes lighthouse@12.8.2 http://127.0.0.1:4173/ --only-categories=accessibility --chrome-flags='--headless --no-sandbox' --preset=desktop --output=json --output-path=lighthouse.json --quiet
python3 -c 'import json; score=json.load(open("lighthouse.json"))["categories"]["accessibility"]["score"]; print(f"accessibility={score:.0%}"); assert score >= 0.95'
# Keep the same external-link allowance and changelog exclusion as Pages links.
lychee --no-progress --accept 200,429 --exclude '^https://bolens[.]github[.]io/appicon/changelog/$' '_site/**/*.html'
kill "$server"
trap - EXIT)
rm -rf .woodpecker-out/site
cp -R "$work/_site" .woodpecker-out/site
