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

The nixpkgs inputs follow the maintained `nixpkgs-26.05-darwin` branch to retain
Intel macOS alongside ARM macOS and both Linux architectures. Nixpkgs 26.11
[removed Intel macOS support](https://nixos.org/manual/nixpkgs/unstable/release-notes#x86_64-darwin-26.11);
review the supported platforms before moving to a newer series.

Source packages use the Go compiler selected by `flake.lock`; the development
shell has its own compiler input in `devenv.lock`. Update the owning nixpkgs
inputs together when applying Go security fixes, then build and check the source
packages. Inspect the compiler with:

```bash
nix eval --raw .#packages.x86_64-linux.appicon.go.version
```

`appicon-bin` repackages an existing published release. Refreshing Nix inputs
cannot change the Go standard library embedded in that binary; updating it
requires a new upstream release and its verified asset hashes.

Home Manager: import `homeManagerModules.default`, set `programs.appicon.enable = true`, and add `overlays.default` (or set `programs.appicon.package` to `appicon` / `appicon-bin` / `appicon-git`).

Optional warm-cache daemon (Linux/systemd):

```nix
programs.appicon.daemon.enable = true;
```

Declarative sources / overrides / BYOK env (JSON by default; set `configFormat = "yaml"` for YAML):

```nix
programs.appicon = {
  enable = true;
  configFormat = "yaml";
  sources = {
    sources = [
      { type = "overrides"; }
      { type = "xdg"; }
      { type = "logo-dev"; token_env = "LOGO_DEV_TOKEN"; }
      { type = "svgl"; }
    ];
  };
  overrides = {
    "my-wm-class" = "firefox";
  };
  # Prefer EnvironmentFile with secret *values* (not sops .path strings):
  # sops.templates."appicon.env".content = ''
  #   LOGO_DEV_TOKEN=${config.sops.placeholder."logo-dev-token"}
  # '';
  environmentFiles = [ config.sops.templates."appicon.env".path ];
};
```

Do **not** set `environment.LOGO_DEV_TOKEN = config.sops.secrets….path` — that puts a filesystem path into the env var `token_env` reads as the API token.

That installs a user socket at `$XDG_RUNTIME_DIR/appicon.sock` (same as `contrib/systemd/`). Source packages also ship units under `$out/lib/systemd/user/`.

`vendorHash` in [packages.nix](packages.nix) / [flake.nix](../flake.nix) is set from a local `go mod vendor` SRI hash. After `go.sum` changes, refresh with:

```bash
go mod vendor
nix hash path vendor
# paste into flake.nix vendorHash, then rm -rf vendor
nix flake lock   # needs network once
nix build
```

On each verified GitHub release, the packaging generator updates `version` and
the `binHashes` in [packages.nix](packages.nix) from accepted assets. These describe
the last published package group; prepared source `VERSION` may advance first.
The fixed-version source package retains that published package version, and
`appicon-git` adds its revision suffix. Tagged release archives instead embed the
exact release tag. Do not point `appicon-bin` at an unpublished version or invent
future hashes. See [the release playbook](../RELEASING.md).

CI: `make check-packaging-versions` aligns flake / AUR / Nix hashes; the `packaging-nix-build` matrix runs `nix build .#{appicon,appicon-git,appicon-bin}`.

## See also

- [Documentation map](../docs/README.md)
- [Root README](../README.md) · [AUR packaging](../packaging/aur/README.md) · [SECURITY.md](../SECURITY.md)
