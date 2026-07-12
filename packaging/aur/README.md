# AUR packaging

Reference PKGBUILDs for publishing to [aur.archlinux.org](https://aur.archlinux.org):

| Package | Path | Notes |
|---------|------|-------|
| `appicon` | [appicon/PKGBUILD](appicon/PKGBUILD) | Build from tagged source |
| `appicon-bin` | [appicon-bin/PKGBUILD](appicon-bin/PKGBUILD) | Install release tarball |

## Publish checklist

1. Tag / GitHub release (`v*`) with `SHA256SUMS` (and optional cosign bundle).
2. Set `pkgver` to the release without the `v` prefix.
3. Fill `sha256sums` / `sha256sums_*` (never leave `SKIP` on the AUR).
4. `makepkg --printsrcinfo > .SRCINFO`
5. Push to `ssh://aur@aur.archlinux.org/<pkgname>.git`

These trees are **reference copies** in this repo — the AUR git repos are canonical once published.
