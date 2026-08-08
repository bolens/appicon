#!/usr/bin/env bash
# Build portable release archives. Override targets for contract tests with
# APPICON_RELEASE_TARGETS as a space-separated list of GOOS/GOARCH pairs.
set -euo pipefail

version=${1:-}
dist=${2:-dist}
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: $0 vX.Y.Z [dist-dir]" >&2
  exit 2
fi

targets=${APPICON_RELEASE_TARGETS:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"}
work=$(mktemp -d "${TMPDIR:-/tmp}/appicon-release.XXXXXX")
trap 'rm -rf "$work"' EXIT
mkdir -p "$dist"
if [[ -e "$dist/SHA256SUMS" ]] || compgen -G "$dist/*.tar.gz" >/dev/null; then
  echo "release output directory contains existing artifacts: $dist" >&2
  exit 2
fi

for target in $targets; do
  goos=${target%/*}
  goarch=${target#*/}
  if [[ -z "$goos" || -z "$goarch" || "$goos" = "$target" ]]; then
    echo "invalid release target: $target" >&2
    exit 2
  fi

  stage="$work/${goos}_${goarch}"
  mkdir -p "$stage/completions" "$stage/man/man1"
  binary=appicon
  [[ "$goos" = windows ]] && binary=appicon.exe
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
    -ldflags "-X github.com/bolens/appicon/internal/version.Version=${version}" \
    -o "$stage/$binary" ./cmd/appicon

  cp LICENSE README.md "$stage/"
  cp internal/completion/appicon.bash "$stage/completions/"
  cp internal/completion/appicon.zsh "$stage/completions/"
  cp internal/completion/appicon.fish "$stage/completions/"
  cp internal/completion/appicon.1 "$stage/man/man1/"
  members=("$binary" LICENSE README.md completions man)
  if [[ "$goos" = linux ]]; then
    mkdir -p "$stage/contrib/systemd"
    cp contrib/systemd/appicon.service contrib/systemd/appicon.socket contrib/systemd/README.md \
      "$stage/contrib/systemd/"
    members+=(contrib)
  fi
  tar -C "$stage" -czf "$dist/appicon_${version}_${goos}_${goarch}.tar.gz" "${members[@]}"
done

(cd "$dist" && sha256sum ./*.tar.gz > SHA256SUMS)
