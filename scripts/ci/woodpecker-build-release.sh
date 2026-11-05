#!/usr/bin/env bash
# Build only. Publication credentials are deliberately absent from this step.
set -euo pipefail
[[ "$CI_FORGE_TYPE" = github && "$CI_REPO" = "bolens/appicon" && "$CI_PIPELINE_EVENT" = tag ]]
[[ "$CI_COMMIT_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
[[ "$(git rev-parse HEAD)" = "$CI_COMMIT_SHA" ]]
[[ -z "$(git status --porcelain --untracked-files=no)" ]]
mkdir -p .woodpecker-out
[[ ! -e .woodpecker-out/release ]]
mkdir .woodpecker-out/release
bash scripts/ci/build-release.sh "$CI_COMMIT_TAG" .woodpecker-out/release
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
git clone --depth 1 --branch 16.15.0 https://github.com/simple-icons/simple-icons.git "$work/simple-icons"
git clone --depth 1 --branch main https://github.com/homarr-labs/dashboard-icons.git "$work/dashboard-icons"
python3 - "$work" <<'PACKS'
import json, subprocess, sys
from pathlib import Path
root = Path(sys.argv[1])
(root / 'NOTICE').write_text('Appicon optional packs bundle. Retain upstream LICENSE files.\nInstall: appicon pack install --from-bundle appicon-packs-bundle.tar.gz\n')
records = {name: subprocess.check_output(['git', '-C', str(root / name), 'rev-parse', 'HEAD'], text=True).strip()
           for name in ['simple-icons', 'dashboard-icons']}
(root / 'SOURCES.json').write_text(json.dumps(records, indent=2) + '\n')
PACKS
rm -rf "$work/simple-icons/.git" "$work/dashboard-icons/.git"
tar -C "$work" -czf .woodpecker-out/release/appicon-packs-bundle.tar.gz NOTICE SOURCES.json simple-icons dashboard-icons
printf 'Appicon %s. Built from %s; see checksums, signed bundle and provenance.\n' "$CI_COMMIT_TAG" "$CI_COMMIT_SHA" > .woodpecker-out/release-notes.md
python3 - <<'MANIFEST'
import hashlib, json, os
from pathlib import Path
out = Path('.woodpecker-out/release')
provenance = {
    '_type': 'https://in-toto.io/Statement/v1',
    'subject': [{'name': f.name, 'digest': {'sha256': hashlib.sha256(f.read_bytes()).hexdigest()}}
                for f in sorted(out.iterdir())],
    'predicateType': 'https://slsa.dev/provenance/v1',
    'predicate': {'buildDefinition': {
        'buildType': 'https://woodpecker-ci.org/ci',
        'externalParameters': {'repository': os.environ['CI_REPO'], 'tag': os.environ['CI_COMMIT_TAG']},
        'resolvedDependencies': [{'uri': 'git+https://github.com/' + os.environ['CI_REPO'],
                                  'digest': {'gitCommit': os.environ['CI_COMMIT_SHA']}}]},
        'runDetails': {'builder': {'id': os.environ['CI_SYSTEM_URL']},
                       'metadata': {'invocationId': os.environ['CI_PIPELINE_URL']}}},
}
(out / 'woodpecker-provenance.json').write_text(json.dumps(provenance, indent=2) + '\n')
with (out / 'SHA256SUMS').open('w') as sums:
    for f in sorted(out.iterdir()):
        if f.name != 'SHA256SUMS':
            sums.write(hashlib.sha256(f.read_bytes()).hexdigest() + '  ' + f.name + '\n')
MANIFEST
(cd .woodpecker-out/release && sha256sum --check --strict SHA256SUMS)
python3 scripts/ci/woodpecker-publish.py manifest
