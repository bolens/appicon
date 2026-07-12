# Deferred ideas

Not a backlog — revisit only if a real consumer needs it.

| Idea | Why deferred | If revisited |
|------|----------------|--------------|
| System-wide tray SNI replacement | StatusNotifierItem icons are owned by apps/compositors; swapping them needs a tray host or hacks | Separate tool for *our* panels only; never claim to be a system SNI host without full D-Bus APIs |
| `appicon self-update` | Extra updater channel to secure | Prefer install script / AUR / Nix; if added: verify `SHA256SUMS` (+ cosign), atomic replace, refuse when Nix/pacman-owned |
| Perfect long-tail app coverage | Infinite proprietary WM classes / broken `.desktop` files | `overrides.json` + packs; miss → exit `1` / glyph fallback is supported; new heuristics only with fixtures from real consumers |

## Operational (not deferred product)

- Publish AUR packages: [packaging/aur/README.md](../packaging/aur/README.md)
- After releases, refresh AUR/Nix checksums: [nix/README.md](../nix/README.md)
- Security reporting and release verification: [SECURITY.md](../SECURITY.md)
