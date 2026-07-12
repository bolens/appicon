# Optional user systemd socket for appicon

```bash
# From a built binary / install:
install -Dm644 contrib/systemd/appicon.socket ~/.config/systemd/user/appicon.socket
install -Dm644 contrib/systemd/appicon.service ~/.config/systemd/user/appicon.service
# Ensure `appicon` is on PATH for the user service, or edit ExecStart to an absolute path.
systemctl --user daemon-reload
systemctl --user enable --now appicon.socket
```

`resolve` dials `$XDG_RUNTIME_DIR/appicon.sock` when present and falls back to in-process
resolve if the socket is missing (`--local` / `APPICON_NO_DAEMON=1` skips the dial).

Foreground without systemd: `appicon daemon` (creates the socket itself).
