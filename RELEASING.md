# Release playbook

[Documentation](docs/README.md)

Appicon publishes Semantic Versioning releases from signed `vX.Y.Z` tags. The
release workflow builds platform archives and checksums, signs the checksum
manifest with keyless Cosign, creates provenance attestations, publishes the
GitHub release, and updates packaging through a squash-merged pull request.

## Woodpecker migration

[The delivery contract](docs/woodpecker-publishing.md) defines the phased native
publisher and host cutover flags. Until cutover, use the Actions procedure below.
After cutover, preserve the same reviewed signed-tag release procedure, follow
the native pipeline, verify published assets and packaging PRs, and retain the
platform exceptions documented there. Gitea reconciliation does not publish.
AUR SSH publication remains manual. Never enable duplicate publishers.

## Prepare

Start a `release/vX.Y.Z` branch from current `origin/main`. Choose the version
from user-visible compatibility impact, update `VERSION`, and move reader-visible
changes from `Unreleased` into a dated `CHANGELOG.md` section. `VERSION` is the
prepared source release; flake/AUR versions and hashes remain the last verified
published package metadata until the post-publication packaging PR. Do not
invent future asset hashes or create a tag during preparation.

```sh
make check
scripts/ci/check-release-build.sh
```

Review generated changes, release notes, archive contents, and packaging URLs.
Never include provider credentials, cache contents, or local indexes.

## Review and merge

Do not push directly to `main`. Open a pull request, wait for every required check, resolve all conversations,
and squash-merge. Direct pushes and protection bypasses are not release paths.
Confirm CI succeeds on the resulting `main` commit before tagging it.

## Publish

After all required CI passes on the exact merged `main` SHA, check out local
`main` at that SHA. Configure a signing key registered with GitHub and local tag
verification. The helper refreshes `origin/main`, requires the explicit validated
SHA and matching `VERSION`, reruns `make check`, and creates and verifies a signed
annotated tag. It never pushes. Push only that tag; the selected publisher owns
assets and packaging updates.

```sh
bash scripts/ci/cut-release.sh vX.Y.Z <full-validated-main-sha>
git push origin refs/tags/vX.Y.Z:refs/tags/vX.Y.Z
```

After native cutover, follow the tag's Woodpecker validation, release build and
publisher, then the retained GitHub OIDC signing/attestation and packaging merge
coordinator. Native `woodpecker-build-release.sh` includes the optional packs
bundle before checksums and provenance. In legacy mode only, follow the hosted
Release and automatic Packs Bundle workflows. Never run duplicate publishers.

## Verify

Verify the GitHub release, checksums, Cosign signature, provenance attestations,
and the automated packaging pull request. Install one published archive in an
isolated location and exercise version plus offline cache-hit and miss behavior.
Confirm the Pages site reports the published release.

## Recover

Fix a failed pre-publication workflow on a new commit before retrying. Never
move a published tag. Correct a public failure with a new patch release; repair
packaging through its generated pull request rather than editing release assets
in place.

Fleet policy: <https://github.com/bolens/.github/blob/main/RELEASING.md>.

## Source lint

The Source lint workflow checks maintained python, javascript, css, shell files selected by
[`.github/source-lint.json`](.github/source-lint.json) on every pull request
and push to `main`. Existing native checks remain part of the merge gate.
Use the [shared local reproduction instructions](https://github.com/bolens/.github/blob/7603518f305fb76f7bb1b9979f2692521f633b82/docs/source-lint.md)
with the same tooling revision pinned in
[the workflow](.github/workflows/source-lint.yml). Review exclusions when adding
source files; generated and imported files retain their native validation.
Require the new check to pass on the current PR head before merging.

Native publisher enablement and rollback coordinate the GitHub flag with the
Woodpecker `config_file` profile. Follow the [profile and drain sequence](docs/woodpecker-publishing.md#validation-only-and-publishing-profiles);
the publishing flag alone is not a complete rollback.
