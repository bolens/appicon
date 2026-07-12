# Nix packaging

AUR-parity attrs (same three install styles as [packaging/aur/](../packaging/aur/)):

| Attr | Like AUR | Install |
|------|----------|---------|
| `appicon` (default) | `appicon` | `buildGoModule` from this flake, fixed `version` in `flake.nix` |
| `appicon-bin` | `appicon-bin` | Prebuilt GitHub release tarball (linux amd64/arm64 only) |
| `appicon-git` | `appicon-git` | `buildGoModule` from this flake with `…-unstable-<rev>` |

```bash
# After installing Nix:
nix flake lock          # writes flake.lock (committed for reproducible inputs)
nix build               # → appicon (source)
nix build .#appicon-bin # linux; needs network to fetch release
nix build .#appicon-git
nix run . -- version
nix profile install .
nix profile install .#appicon-bin
```

Home Manager: import `homeManagerModules.default`, set `programs.appicon.enable = true`, and add `overlays.default` (or set `programs.appicon.package` to `appicon` / `appicon-bin` / `appicon-git`).

Optional warm-cache daemon (Linux/systemd):

```nix
programs.appicon.daemon.enable = true;
```

That installs a user socket at `$XDG_RUNTIME_DIR/appicon.sock` (same as `contrib/systemd/`). Source packages also ship units under `$out/lib/systemd/user/`.

`vendorHash` in [packages.nix](packages.nix) / [flake.nix](../flake.nix) is set from a local `go mod vendor` SRI hash. After `go.sum` changes, refresh with:

```bash
go mod vendor
nix hash path vendor
# paste into flake.nix vendorHash, then rm -rf vendor
nix flake lock   # needs network once
nix build
```

On each GitHub release, update `version` and the `binHashes` in [packages.nix](packages.nix) from `SHA256SUMS` (convert with `nix hash convert --hash-algo sha256 --to sri <hex>`).

CI: `make check-packaging-versions` aligns flake / AUR / Nix hashes; the `packaging-nix-build` matrix runs `nix build .#{appicon,appicon-git,appicon-bin}`.

## See also

- [Documentation map](../docs/README.md)
- [Root README](../README.md) · [AUR packaging](../packaging/aur/README.md) · [SECURITY.md](../SECURITY.md)
