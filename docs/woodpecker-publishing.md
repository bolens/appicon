# Woodpecker delivery

[Documentation](README.md) · [Release playbook](../RELEASING.md) · [Contributing](../CONTRIBUTING.md)

## Authority and migration

GitHub `bolens/appicon` is the canonical release source. The direct GitHub forge
on Woodpecker validates the exact GitHub commit. The reconciled Gitea copy keeps
its visibility, governance adoption history and protected squash delivery; it
validates independently and never receives publisher credentials or publishes a
second copy. This authorization moves the publisher, not source ownership.

Both Linux Docker agents can run the workflow. Its containers have no host Docker
socket, added capabilities, host volumes or registry credentials. Native macOS
`devenv` checks remain in GitHub Actions. Existing Linux Docker/devenv checks also
remain there until a safe dedicated builder is available. GitHub metadata
assignment and Spec Kit update PR automation remain in Actions.

Migration has two explicit host settings. Until the coordinator changes them,
existing Actions jobs continue and publication remains disabled in Woodpecker:

| Repository variable | Cutover condition | Effect |
| --- | --- | --- |
| `WOODPECKER_LINUX_CI=true` | Exact candidate passes native checks; required check settings migrated | Retires portable Linux Actions jobs, preserving hosted exceptions |
| `WOODPECKER_PUBLISHING=true` | Native validation green, scoped credential installed, Pages branch configured, publication contract reviewed | Selects Woodpecker as the sole release and Pages builder/publisher |

Never flip these flags solely because YAML parses. Require a real GitHub-forge
pull-request and main pipeline, verify external status contexts and the retained
macOS job, and exercise credential-free release/Pages contracts. Existing public
versions are immutable. This migration creates no tag, release or AUR publication.

## Credentials and event boundaries

The only pipeline secret is `github_publish_token`, mapped to `GH_TOKEN` in the
two publisher steps. Provision it through the server-signed credential broker only for the native
GitHub repository ID and verified protected `push`/`tag` refs and commits. Do not enable it for pull
requests, manual builds, cron, Gitea repositories, owners or the whole instance.
For command helpers, native image filters are intentionally omitted: Woodpecker
3.18 permits those filters only for plugin steps. The server-verified forge,
repository remote ID, protected ref and exact commit are the credential boundary.
Use short-lived selected-repository App tokens. The script independently
requires the GitHub forge type/URL, exact repository,
matching checkout and event, clean tracked source and explicit cutover flag.

Prefer a GitHub App installed only on this repository with Contents write,
Pull requests write, Variables read and Metadata read. Appicon additionally needs
Actions write for its OIDC bridge; retained platform validation may need Checks
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
Any existing version causes failure; do not delete or overwrite public artifacts
to retry. After publication it downloads each native asset and compares its
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

The packaging phase generates hash updates on fresh canonical main and opens a
normal pull request. Require current native checks, review and protected squash
merge; this credential does not grant a main protection bypass. Actual
`aur.archlinux.org` pushes remain the existing explicit maintainer operation.
No AUR account ownership or SSH key is assumed by this pipeline.

## Pages

Woodpecker assembles and checks the site in a disposable source copy using the
latest published release metadata. Existing accessibility, links and browser
checks remain gates. On a current canonical main push, the publisher appends a
commit containing the generated site and `.nojekyll` to `gh-pages`, using no force
push. Concurrent or superseded deployments fail rather than overwriting history.
Root configures Pages to serve that branch, preserving the existing public URL.
GitHub's branch-based Pages hosting is a platform service, not a second custom
site builder. To refresh release metadata, run the next reviewed main change;
release publication itself does not create an unreviewed main commit.

## Verification and recovery

Run `python3 -m unittest discover -s scripts/ci -p test_woodpecker_publication.py`,
`shellcheck scripts/ci/woodpecker-*.sh`, `actionlint`, the repository native gates
and the shared workflow-security/source-lint checks. Tests use mocked host APIs;
they do not publish. Native PR execution proves the runtime path. Release OIDC
and public publication remain unexercised until a real authorized release.

On pre-publication failure, retain the draft and private receipt, diagnose the
failing stage and inspect immutable asset identities before an explicit recovery.
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
