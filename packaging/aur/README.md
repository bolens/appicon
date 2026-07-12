# AUR packaging

Reference PKGBUILDs for publishing to [aur.archlinux.org](https://aur.archlinux.org):

| Package | Path | Notes |
|---------|------|-------|
| `appicon` | [appicon/PKGBUILD](appicon/PKGBUILD) | Build from tagged source |
| `appicon-bin` | [appicon-bin/PKGBUILD](appicon-bin/PKGBUILD) | Install release tarball |
| `appicon-git` | [appicon-git/PKGBUILD](appicon-git/PKGBUILD) | Build from latest `main` commit |

All three `provides=('appicon')` and conflict with each other — install only one.

## Publish checklist (stable / bin)

1. Tag / GitHub release (`v*`) with `SHA256SUMS` (and optional cosign bundle).
2. Set `pkgver` to the release without the `v` prefix.
3. Fill `sha256sums` / `sha256sums_*` (never leave `SKIP` on the AUR).
4. `makepkg --printsrcinfo > .SRCINFO`
5. Push to `ssh://aur@aur.archlinux.org/<pkgname>.git`

## Publish checklist (git)

1. Clone `ssh://aur@aur.archlinux.org/appicon-git.git`
2. Copy [appicon-git/PKGBUILD](appicon-git/PKGBUILD); `sha256sums=('SKIP')` is correct for VCS.
3. `makepkg -o` once so `pkgver()` refreshes, then `makepkg --printsrcinfo > .SRCINFO`
4. Push; AUR helpers rebuild from tip of upstream `main` on each update

These trees are **reference copies** in this repo — the AUR git repos are canonical once published.
