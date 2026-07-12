# Icon packs

Local icon trees registered as `pack` stages in [sources.md](sources.md).

Part of the [documentation map](README.md). Stages/order: [sources.md](sources.md). Consumer contract: [consumer-contract.md](consumer-contract.md).

## Layout

Recommended root: `$XDG_DATA_HOME/appicon/packs/<name>/` (default `~/.local/share/appicon/packs/`).

Resolution (see `internal/pack`):

1. Optional `index.json` map of query → relative path
2. Exact stem match on `.svg` / `.png` / `.webp`
3. Fuzzy stem match

## CLI

```bash
appicon pack path
appicon pack list [--json]
appicon pack add <name> <dir>
appicon pack install simple-icons|dashboard-icons [--path DIR]
appicon pack install --name NAME [--subdir DIR] [--ref REF] https://github.com/org/icons.git
appicon pack install --name NAME [--subdir DIR] https://example.com/pack.tar.gz
appicon pack update [recipe]
appicon pack install --from-bundle ./appicon-packs-bundle.tar.gz
```

`pack install` accepts (put flags before the recipe/URL):

| Target | Behavior |
|--------|----------|
| Recipe (`simple-icons`, `dashboard-icons`) | Shallow-clone pinned upstream, register pack |
| Git URL (`https://…`, `git@…`, `file://…`) | Shallow-clone into `$XDG_DATA_HOME/appicon/packs/<name>` |
| Archive URL (`*.tar.gz` / `*.tgz` / `*.tar`) | Download, extract, register |

Flags: `--name`, `--path` (destination), `--subdir` (pack root inside clone), `--ref` (branch/tag). Requires `git` for clones; refuses network when offline.

| Recipe | Upstream | Pack subdir |
|--------|----------|-------------|
| `simple-icons` | simple-icons/simple-icons | `icons/` |
| `dashboard-icons` | homarr-labs/dashboard-icons | repo root |

## CDN vs local pack

- **Local pack** (`pack install` / `pack add`): files on disk; works offline after clone.
- **CDN stage** (`simple-icons` / `dashboard-icons` in `sources.json`): opt-in network fetch into cache; no git clone.

You can use both (e.g. local pack first, CDN as supplement).

## Bundle artifact

Optional separate release asset `appicon-packs-bundle.tar.gz` (not inside the main binary tarball). Extract/register with `pack install --from-bundle`.

Bump recipe pins in `internal/packs/packs.go` (`Recipes`) when refreshing clones; the packs-bundle CI workflow clones the same pins.

## MCP

| Tool | Notes |
|------|--------|
| `pack_list` / `pack_path` / `pack_add` | List / root / register local dir |
| `pack_install` | `recipe` **or** `url` (git / `.tar.gz`); optional `name`, `subdir`, `ref`, `path`, `offline` |
| `pack_update` | Optional `recipe` filter; `offline` |
| `pack_install_bundle` | Local `.tar.gz` path on the MCP host |

Mutative tools are documented as such in tool descriptions. Prefer these over shelling `appicon` when MCP is connected.

## See also

- [Documentation map](README.md)
- [sources.md](sources.md) — `pack` stage in `sources.json`
- [consumer-contract.md](consumer-contract.md)
- [deferred.md](deferred.md)
- [../README.md](../README.md) · [../AGENTS.md](../AGENTS.md)
