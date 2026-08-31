# Release playbook

Use this checklist for every stable release. Releases are made from a reviewed,
green `main` commit; never tag an unmerged branch.

## 1. Prepare and merge

1. Update the release branch from current `origin/main` and run `make check` plus
   `go test -race ./...`.
2. Move the relevant `CHANGELOG.md` entries from `Unreleased` into a dated
   `X.Y.Z` section. Do not update AUR/Nix binary hashes yet: release assets do
   not exist until the tag workflow completes.
3. Push the branch, open a PR, and address all actionable review comments,
   including Copilot suggestions. Resolve conversations only after the fix is
   pushed or the rationale is recorded.
4. Wait for the required **CI result** check and every other required check to
   pass. Merge the PR and update the local `main` with `git pull --ff-only`.

## 2. Tag and verify

```bash
bash scripts/ci/cut-release.sh vX.Y.Z
git push origin vX.Y.Z
```

The helper requires a clean tree and reruns `make check`. The tag triggers the
release workflow, which waits for CI on that exact commit, builds Linux amd64
and arm64 archives, publishes checksums and a keyless signature bundle, and
creates the GitHub release.

Wait for both the Release and Packs Bundle workflows. Confirm the optional
`appicon-packs-bundle.tar.gz` appears in `gh release view vX.Y.Z`; the bundle
workflow waits for publication and must fail rather than silently skip the
attachment. Then download all release assets and verify them:

```bash
gh release download vX.Y.Z --dir /tmp/appicon-vX.Y.Z
bash scripts/ci/verify-release.sh /tmp/appicon-vX.Y.Z
```

Confirm `gh release view vX.Y.Z` reports a published, non-draft release.

## 3. Confirm the packaging update

The release workflow calculates the tagged source hash and both Linux archive
hashes. It opens a packaging PR, dispatches CI for that branch, and squash-merges
the PR after CI passes. A normal release has no manual hash step.

Confirm that the packaging PR merged and that `main` contains the released
version in `flake.nix`, the AUR files, and `nix/packages.nix`. If automation
fails, run the updater locally with hashes from the published assets:

```bash
python3 scripts/ci/update-release-packaging.py X.Y.Z SOURCE_SHA AMD64_SHA ARM64_SHA
make check-packaging-versions
make check
```

Submit the recovery changes as a packaging PR. Publishing the canonical AUR
repositories remains a maintainer action; this repository contains reference
copies.

## 4. Update a local binary installation

For a binary installed in `~/.local/bin`, select the archive matching
`uname -m` (`x86_64` → `amd64`, `aarch64`/`arm64` → `arm64`), verify the assets
as above, then install atomically:

```bash
install -Dm755 /tmp/appicon-vX.Y.Z/appicon ~/.local/bin/appicon.new
mv ~/.local/bin/appicon.new ~/.local/bin/appicon
appicon version
```

If appicon is managed by Nix, Home Manager, or an AUR helper, update it through
that package manager after the packaging PR lands instead of replacing its
binary manually.

## Recovery

- Do not reuse or move a published tag. Correct the problem on `main` and cut a
  patch release.
- If the tag workflow fails before publishing, fix the workflow/code on `main`
  and use a new version unless the failed tag and GitHub release were never
  externally visible.
- Keep downloaded assets until checksum/signature verification and the local
  version check both succeed.
- If the packaging PR fails CI, fix that PR and squash-merge it. The GitHub
  release is already published, so do not move or reuse the tag.

## See also

- [Documentation map](README.md)
- [Project README](../README.md)
- [Changelog](../CHANGELOG.md)
- [AUR publishing](../packaging/aur/README.md)
- [Nix packaging](../nix/README.md)
