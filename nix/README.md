# Nix packaging

```bash
# After installing Nix:
nix flake lock          # writes flake.lock
nix build               # fix vendorHash from the error hint, then rebuild
nix run . -- version
nix profile install .
```

Home Manager: import `homeManagerModules.default`, set `programs.appicon.enable = true`, and add `overlays.default` (or set `programs.appicon.package` explicitly).

`vendorHash` in [flake.nix](../flake.nix) is set from a local `go mod vendor` SRI hash. After `go.sum` changes, refresh with:

```bash
go mod vendor
nix hash path vendor
# paste into flake.nix vendorHash, then rm -rf vendor
nix flake lock   # needs network once
nix build
```
