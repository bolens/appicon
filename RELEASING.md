# Release playbook

Appicon publishes Semantic Versioning releases from signed `vX.Y.Z` tags. The
release workflow builds platform archives and checksums, signs the checksum
manifest with keyless Cosign, creates provenance attestations, publishes the
GitHub release, and updates packaging through a squash-merged pull request.

## Prepare

Start a `release/vX.Y.Z` branch from current `origin/main`. Choose the version
from user-visible compatibility impact, update every version source with the
repository release tooling, and move reader-visible changes from `Unreleased`
into a dated `CHANGELOG.md` section. Do not hand-edit generated packaging data.

```sh
scripts/ci/cut-release.sh vX.Y.Z
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

Create a signed annotated `vX.Y.Z` tag on the validated `main` SHA and push only
that tag. The workflow is authoritative for assets and packaging updates.

```sh
git tag -s vX.Y.Z <validated-sha> -m 'appicon X.Y.Z'
git push origin vX.Y.Z
gh run watch --exit-status "$(gh run list --workflow Release --limit 1 --json databaseId --jq '.[0].databaseId')"
```

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
