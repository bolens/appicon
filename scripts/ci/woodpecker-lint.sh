#!/usr/bin/env bash
set -euo pipefail
python3 scripts/ci/sync-woodpecker-validation.py --check
python3 -m unittest discover -s scripts/ci -p test_woodpecker_validation_profile.py
out="$PWD/.woodpecker-out"
mkdir -p "$out"
python3 -m venv "$out/lint-venv"
"$out/lint-venv/bin/pip" install --disable-pip-version-check ruff==0.15.20 zizmor==1.29.0
export PATH="$out/lint-venv/bin:$PATH"
# Match the existing immutable shared-workflow implementations.
git init "$out/source-tooling"
git -C "$out/source-tooling" remote add origin https://github.com/bolens/.github.git
git -C "$out/source-tooling" fetch --depth 1 origin 7603518f305fb76f7bb1b9979f2692521f633b82
git -C "$out/source-tooling" checkout --detach FETCH_HEAD
npm ci --ignore-scripts --prefix "$out/source-tooling/tools/source-lint"
python3 "$out/source-tooling/scripts/lint-source.py" "$PWD"
git init "$out/spec-tooling"
git -C "$out/spec-tooling" remote add origin https://github.com/bolens/.github.git
git -C "$out/spec-tooling" fetch --depth 1 origin 7159a187479e511a9420f8f4eec93246a43fea12
git -C "$out/spec-tooling" checkout --detach FETCH_HEAD
python3 "$out/spec-tooling/scripts/validate-spec-kit.py" --expected-version current .
curl --fail --location --retry 3 --output "$out/actionlint.tar.gz" https://github.com/rhysd/actionlint/releases/download/v1.7.12/actionlint_1.7.12_linux_amd64.tar.gz
printf '%s  %s\n' 8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8 "$out/actionlint.tar.gz" | sha256sum --check --strict
tar -xzf "$out/actionlint.tar.gz" -C "$out" actionlint
"$out/actionlint" -color
zizmor --offline --min-severity medium --min-confidence medium .github
python3 -m unittest discover -s scripts/ci -p test_woodpecker_publication.py
