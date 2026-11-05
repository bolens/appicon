# Woodpecker delivery

[Documentation](README.md) · [Release playbook](../RELEASING.md) · [Contributing](../CONTRIBUTING.md)

Native push validation runs only on `main`; feature branches use their pull-request
pipeline. Existing manual, tag and cron triggers remain available.

## Authority and migration

GitHub `bolens/appicon` is the canonical release source. The direct GitHub forge
on Woodpecker validates the exact GitHub commit. The reconciled Gitea copy keeps
its visibility, governance adoption history and protected squash delivery; it
validates independently and never receives publisher credentials or publishes a
second copy. This authorization moves the publisher, not source ownership.

Both Linux Docker agents can run the workflow. Its containers have no host Docker
socket, added capabilities, host volumes or registry credentials. Native macOS
`devenv` checks and a thin forge-only packaging merge coordinator remain in
GitHub Actions. The latter performs no build, test suite or release publication. Existing Linux Docker/devenv checks also
remain there until a safe dedicated builder is available. GitHub metadata
assignment and Spec Kit update PR automation remain in Actions. Optional manual
pack-bundle artifact storage remains there until a native retained-artifact
consumer is available; it cannot attach release assets after publisher cutover.

Migration has three explicit host settings. Until the coordinator changes them,
existing Actions jobs continue and publication remains disabled in Woodpecker:

| Repository variable | Cutover condition | Effect |
| --- | --- | --- |
| `WOODPECKER_LINUX_CI=true` | Exact candidate passes native checks; required check settings migrated | Retires portable Linux Actions jobs only when publication is also native, preserving hosted exceptions |
| `WOODPECKER_PUBLISHER_ID` | Immutable user ID of the approved App publisher | Rejects foreign draft/release authors |
| `WOODPECKER_PUBLISHING=true` | Native validation green, scoped credential installed, Pages branch configured, publication contract reviewed | Selects Woodpecker as the sole release and Pages builder/publisher |

Move publication first while both providers still validate; retire portable Linux
Actions only after native contexts pass and required checks have migrated. A
half-cutover keeps the old Linux jobs so the old release gate cannot lose CI.
Never flip these flags solely because YAML parses. Require a real GitHub-forge
pull-request and main pipeline, verify external status contexts and the retained
macOS job, and exercise credential-free release/Pages contracts. Existing public
versions are immutable. This migration creates no tag, release or AUR publication.

## Credentials and event boundaries

The pipeline receives only `github_publish_lease`, mapped to `PUBLISHER_LEASE`
in the two publisher steps. The server-signed broker binds it to the native
GitHub repository ID and verified protected `push`/`tag` refs and exact commits.
Do not issue it for pull requests, feature refs, manual builds, cron, Gitea
repositories, owners or the whole instance.

The helper redeems it just before GitHub operations at the fixed HTTPS endpoint
`https://woodpecker.bolens.dev/publisher/credentials`, posting `lease`,
`repository`, `commit`, `purpose=github` and a fresh `request_id`. The broker
compares the complete stored binding. Its lease lasts at most 12 hours, with
at most eight token mints per pipeline. The helper caches `token`/RFC3339
`expires_at` in memory and refreshes when fewer than 120 seconds remain. Only a
fresh App token reaches a child process as `GH_TOKEN`; the lease, inherited GitHub
credentials and debug/trace environment are removed. No App private key enters
the runner.

For command helpers, native image filters are intentionally omitted: Woodpecker
3.18 permits those filters only for plugin steps. The broker's authenticated
forge, repository remote ID, protected ref and exact commit are the credential
boundary. The script independently requires the GitHub forge type/URL, exact
repository, matching checkout and event, clean tracked source and cutover flag.

Prefer a GitHub App installed only on this repository with Contents write,
Pull requests write, Variables read and Metadata read. Both repositories need
Actions write for merge-coordinator dispatch; Appicon also uses it for the OIDC
bridge; retained platform validation may need Checks
and Commit statuses read. Credentials go through the secret store, never Git,
URL parameters, logged commands, clone configuration or uploaded artifacts.
There is no OCI registry consumer or package registry publication in this repo.

## Release transaction

A maintainer first prepares and reviews a release, verifies current main checks,
and creates the existing signed annotated `vX.Y.Z` tag on the validated GitHub
commit. Push that tag once. Woodpecker runs validation before its build and
publication steps. The publisher checks GitHub's signature verification, tag
object and commit, and confirms the commit belongs to canonical main.

The build step receives no publisher secret. It records exact source, tag,
builder invocation, asset hashes and SLSA-format provenance. The publisher creates
a draft with an explicit commit target and uploads assets without replacement.
A matching draft can resume after its approved author, source, original build
receipt and every existing asset hash are verified. Upload only missing assets
whose bytes match that receipt. Published versions are never repaired in place;
a matching complete publication may resume only packaging. Do not delete or
overwrite public artifacts to retry. After publication it downloads each native asset and compares its
SHA-256 digest with the locally built receipt.

Appicon builds all six existing Go OS/architecture archives and the optional
packs bundle. The bundle records resolved upstream recipe commits and retains
upstream licenses. `release.yml` retains only the GitHub OIDC signing/attestation
bridge after cutover. Woodpecker dispatches it at the exact tag. The bridge
verifies every native draft asset, signs `SHA256SUMS` with the existing workflow
identity, attests the assets, attaches the signature and publishes the draft.
The existing keyless verification identity remains valid. GitHub's attestation
identifies this signing bridge; the accompanying native provenance identifies the
actual Woodpecker build. Do not describe the bridge as the compiler.

The native packaging phase generates hash updates on fresh canonical main,
refusing a version downgrade, and opens or resumes its normal pull request.
Existing PRs must have canonical numeric repository IDs on both sides, `main`
as base, the deterministic head branch and the approved App author. The helper
regenerates an expected tree on fresh main and verifies that tree and the exact
generated-file allowlist before trusting the PR.

It then dispatches only the thin `release.yml` merge-coordinator job at the
verified signed tag, passing the immutable PR/head/base/tree intent as data, and
exits with a local `packaging_pending` receipt. It does not hold a native runner
while waiting for the PR checks: concurrent releases must leave ci01/ci02 free to
run those checks. A green release-build pipeline is not evidence of a completed
packaging merge.

The retained forge-only job checks its own canonical signed-tag workflow context,
uses only the native job-scoped GitHub token, verifies published native assets,
and regenerates the exact intended packaging tree from protected main. Appicon
also cryptographically verifies the checksum bundle against the existing
tag-specific GitHub OIDC identity before using those hashes for packaging. It never
executes code from the PR checkout. It checks the canonical repo/base/head/App
author again, waits for normal required checks, then rechecks source identity and
requests protected squash merge with `--match-head-commit`. No auto-merge setting,
admin bypass, PR credential lease or webhook service is introduced. Completion is
reported only after the actual merge state and commit are verified. Each workflow
attempt retains a `native-packaging-merge-receipt-*` artifact showing `merged` or
`failed`; use it with the native pending receipt for asynchronous acceptance.

Stale, forked or unrelated branches fail for normal reviewed repair rather than
being force-updated. Completed-PR recovery also requires the historical head tree
to match the current expected tree; unrelated later main changes can therefore
require manual reviewed recovery. Actual `aur.archlinux.org` pushes remain the
explicit maintainer operation; no AUR ownership or SSH key is assumed.

## Pages

Woodpecker assembles and checks the site in a disposable source copy using the
latest published release metadata. Credential-free builds resolve GitHub’s public
`releases/latest` redirect instead of sharing the unauthenticated REST quota,
validate its canonical stable tag, and retry transient failures only three times.
The authenticated publisher rechecks the latest tag immediately before pushing. Existing accessibility, links and browser
checks remain gates. On a current canonical main push, the publisher appends a
commit containing the generated site and `.nojekyll` to `gh-pages`, using no force
push. Concurrent or superseded deployments fail rather than overwriting history.
Root configures Pages to serve that branch, preserving the existing public URL.
GitHub's branch-based Pages hosting is a platform service, not a second custom
site builder. To refresh release metadata, run the next reviewed main change;
release publication itself does not create an unreviewed main commit.

## Verification and recovery

Run `python3 -m unittest discover -s scripts/ci -p test_woodpecker_publication.py`,
`python3 -m unittest discover -s scripts/ci -p test_public_release_metadata.py`,
`shellcheck scripts/ci/woodpecker-*.sh`, `actionlint`, the repository native gates
and the shared workflow-security/source-lint checks. Tests use mocked host APIs;
they do not publish. Native PR execution proves the runtime path. Release OIDC
and public publication remain unexercised until a real authorized release.

On pre-publication failure, retain the draft and private receipt and diagnose the
failing stage. Retry the protected tag pipeline through the broker for a fresh
selected-repository token. It resumes matching completed uploads and the existing
packaging PR. If a partial upload is missing bytes that differ from a rebuild,
recover those original files from the retained build invocation into
`.woodpecker-out/release/`; restore its original `release-manifest.json`, then run
`python3 scripts/ci/woodpecker-publish.py publish` followed by `packaging` in the
same approved tag context with a fresh broker credential. Never replace existing
assets or delete a draft merely to defeat an identity mismatch.

Queued validation does not consume an App token's lifetime: token issuance is
just in time after lease redemption. The cache refreshes through the broker for
long signing/check waits, within the bounded mint/lease budget. Authentication,
expiry or budget failure preserves resumable draft/PR state and reports failure;
it is not a successful publication. Verify the broker path and isolation on the
real protected source before cutover.
Never move published tags. Fix public release defects with a new patch release.
Rollback a cutover only with the coordinator ensuring exactly one publisher is
enabled. Preserve visibility, all domain safety rules and private execution
evidence throughout.

## Pinned input maintenance

Dependabot retains Actions, language manifests where configured, and devcontainer
image coverage. It does not parse these Woodpecker pins or helper download/tool
versions. Review those inputs alongside scheduled fleet maintenance: clone/Nix/
Node/native image digests, checksum-pinned actionlint/lychee, shared tooling
commits and Python tools. Existing runtime/package locks stay authoritative.
Verify upstream release identity and update the native and retained-host
contracts together; an immutable reference alone is not update monitoring.
