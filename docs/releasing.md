# Release playbook

Use this checklist for every stable release. Releases are made from a reviewed,
green `main` commit; never tag an unmerged branch.

## 1. Prepare and merge

1. Rebase the release PR on current `origin/main` and run `make check` plus
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

Wait for both the Release and Packs Bundle workflows. Then download all release
assets and verify them:

```bash
gh release download vX.Y.Z --dir /tmp/appicon-vX.Y.Z
bash scripts/ci/verify-release.sh /tmp/appicon-vX.Y.Z
```

Confirm `gh release view vX.Y.Z` reports a published, non-draft release.

## 3. Refresh packaging

After assets exist, update the stable packaging references on a new branch:

1. Set `version` in `flake.nix` and `pkgver` in the stable and binary AUR
   `PKGBUILD`s.
2. Put the tagged GitHub source archive hash in `packaging/aur/appicon`.
3. Put both release archive hashes in `packaging/aur/appicon-bin`, and convert
   them to SRI for `nix/packages.nix` with
   `nix hash convert --hash-algo sha256 --to sri HASH`.
4. Refresh both `.SRCINFO` files with `makepkg --printsrcinfo` and update the
   `appicon-git` placeholder version when appropriate.
5. Run `make check-packaging-versions`, `make check`, and the available Nix/AUR
   build checks. Submit and merge this as a separate packaging PR.
6. Publish the canonical AUR repositories when configured; the checked-in AUR
   directories are reference copies.

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

## See also

- [Documentation map](README.md)
- [Project README](../README.md)
- [Changelog](../CHANGELOG.md)
- [AUR publishing](../packaging/aur/README.md)
- [Nix packaging](../nix/README.md)
