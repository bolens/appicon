# Appicon native CI and publisher migration

Status: implementation candidate, not operational acceptance.

Move portable Linux validation, release build/upload, packaging PR generation
and Pages assembly/publication to Woodpecker's direct GitHub forge. Preserve
exact canonical source, all existing platform outputs, domain safety, public
URLs, immutability and protected delivery. Native macOS, the safe-builder gap,
GitHub metadata/updater automation and Appicon OIDC stay explicit exceptions.
No synthetic tag/release, no Gitea publisher, no new AUR ownership assumption.

Acceptance: native exact-head PR/main green before either cutover; existing
Actions default active; publisher credentials absent from PR/Gitea; annotated
signed-tag/source/forge/event guards fail closed; immutable asset verification;
existing signing identity retained where applicable; packaging through normal
PRs; generated Pages branch without force pushes. Runtime release publication
is verified only when a real release is separately authorized.
