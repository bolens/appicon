# Nix packaging

AUR-parity attrs (same three install styles as [packaging/aur/](../packaging/aur/)):

| Attr | Like AUR | Install |
|------|----------|---------|
| `appicon` (default) | `appicon` | `buildGoModule` from this flake, fixed `version` in `flake.nix` |
| `appicon-bin` | `appicon-bin` | Prebuilt GitHub release tarball (linux amd64/arm64 only) |
| `appicon-git` | `appicon-git` | `buildGoModule` from this flake with `…-unstable-<rev>` |

```bash
# After installing Nix:
nix flake lock          # writes flake.lock (once; needs network)
nix build               # → appicon (source)
nix build .#appicon-bin # linux; needs network to fetch release
nix build .#appicon-git
nix run . -- version
nix profile install .
nix profile install .#appicon-bin
```

Home Manager: import `homeManagerModules.default`, set `programs.appicon.enable = true`, and add `overlays.default` (or set `programs.appicon.package` to `appicon` / `appicon-bin` / `appicon-git`).

`vendorHash` in [packages.nix](packages.nix) / [flake.nix](../flake.nix) is set from a local `go mod vendor` SRI hash. After `go.sum` changes, refresh with:

```bash
go mod vendor
nix hash path vendor
# paste into flake.nix vendorHash, then rm -rf vendor
nix flake lock   # needs network once
nix build
```

On each GitHub release, update `version` and the `binHashes` in [packages.nix](packages.nix) from `SHA256SUMS` (convert with `nix hash convert --hash-algo sha256 --to sri <hex>`).
