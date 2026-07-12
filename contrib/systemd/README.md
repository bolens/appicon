# Optional user systemd socket for appicon

```bash
# From a built binary / install:
install -Dm644 contrib/systemd/appicon.socket ~/.config/systemd/user/appicon.socket
install -Dm644 contrib/systemd/appicon.service ~/.config/systemd/user/appicon.service
# Ensure `appicon` is on PATH for the user service, or edit ExecStart to an absolute path.
# Prefer Home Manager: programs.appicon.daemon.enable = true (sets ExecStart to the package path).
systemctl --user daemon-reload
systemctl --user enable --now appicon.socket
```

`resolve` and `prefetch` dial `$XDG_RUNTIME_DIR/appicon.sock` when present and fall back to
in-process resolve if the socket is missing (`--local` / `APPICON_NO_DAEMON=1` skips the dial).

The daemon speaks the same resolve path as the CLI: `order`, `explain`, and `resolve-batch`
(multi-query) are supported over the socket — no need to force `--local` for those flags.

Foreground without systemd: `appicon daemon` (creates the socket itself).

## See also

- [Documentation map](../../docs/README.md)
- [Root README](../../README.md) (daemon section) · [nix/README.md](../../nix/README.md) (`programs.appicon.daemon.enable`)
- [docs/consumer-contract.md](../../docs/consumer-contract.md)
