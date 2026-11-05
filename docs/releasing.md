# Release playbook

Use this checklist for every stable release. Releases are made from a reviewed,
green `main` commit; never tag an unmerged branch.

## 1. Prepare and merge

1. Update the release branch from current `origin/main` and run `make check` plus
   `go test -race ./...`.
2. Set `VERSION` to `X.Y.Z` and move the relevant `CHANGELOG.md` entries from
   `Unreleased` into a dated section. Keep flake/AUR versions and binary hashes
   at their last verified published release until the automated packaging PR:
   the future assets do not exist yet. Do not create a tag during preparation.
3. Push the branch, open a PR, and address all actionable review comments,
   including Copilot suggestions. Resolve conversations only after the fix is
   pushed or the rationale is recorded.
4. Wait for every required native and retained hosted check to pass. Squash-merge
   the PR, update local `main`, and require CI on its exact resulting SHA.

## 2. Tag and verify

```bash
bash scripts/ci/cut-release.sh vX.Y.Z <full-validated-main-sha>
git push origin refs/tags/vX.Y.Z:refs/tags/vX.Y.Z
```

Configure a signing key registered with GitHub and local tag verification first.
The helper requires clean local `main`, refreshes canonical `origin/main`, checks
the supplied full SHA and prepared `VERSION`, and reruns `make check`. It refreshes
the remote again before creating and verifying a signed annotated tag on that
exact commit. It never pushes; failed signing or verification requires deliberate
local recovery before any push. Older Git versions can leave an unsigned local
tag after a signing error. Inspect its signature and confirm it is unpublished
before deleting that local tag or retrying. The helper leaves local tag removal to the maintainer. Local checks do not replace authoritative CI.

After native cutover, follow the tag's Woodpecker validation, release build and
publisher, then the retained GitHub OIDC signing/attestation bridge and packaging
merge coordinator. `scripts/ci/woodpecker-build-release.sh` builds the optional
`appicon-packs-bundle.tar.gz` before checksums and provenance; the disabled
automatic hosted Packs Bundle job is not an acceptance gate. In legacy mode only,
wait for hosted Release and automatic Packs Bundle. Confirm the bundle appears in
`gh release view vX.Y.Z`, then download and verify all release assets:

```bash
gh release download vX.Y.Z --dir /tmp/appicon-vX.Y.Z
bash scripts/ci/verify-release.sh /tmp/appicon-vX.Y.Z
```

Confirm `gh release view vX.Y.Z` reports a published, non-draft release.

## 3. Confirm the packaging update

The selected publisher calculates the tagged source hash and both Linux archive
hashes. It opens a packaging PR, dispatches CI for that branch, and squash-merges
the PR after CI passes. A normal release has no manual hash step.

Confirm that the packaging PR merged and that `main` contains the released
version in `flake.nix` and the AUR files, with verified hashes in
`nix/packages.nix`. The generator leaves prepared `VERSION` unchanged and refuses
to downgrade packaging after main prepares a newer release. If automation
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
