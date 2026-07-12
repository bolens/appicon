#compdef appicon

_appicon() {
  local -a cmds cache_cmds override_cmds shells formats themes

  cmds=(
    'resolve:Resolve an icon query to a local path'
    'prefetch:Warm the icon cache for queries'
    'cache:Cache path/clear/stats/prune'
    'override:Manage overrides.json remaps'
    'sources:List effective resolve stage order'
    'pack:Manage local icon packs'
    'daemon:Run optional unix-socket resolve daemon'
    'mcp:Run stdio MCP server for agents'
    'completion:Print shell completion script'
    'man:Print man page (troff) to stdout'
    'version:Print version'
    'help:Show usage'
  )
  cache_cmds=(path clear stats prune)
  override_cmds=(list get set rm path)
  sources_cmds=(list path)
  pack_cmds=(list path add install update)
  shells=(bash zsh fish)
  formats=(svg png)
  themes=(dark light)

  _arguments -C \
    '1:command:->cmd' \
    '*::arg:->args'

  case $state in
    cmd)
      _describe -t commands 'appicon command' cmds
      ;;
    args)
      case $words[1] in
        resolve)
          _arguments \
            '--json[Emit JSON result]' \
            '--offline[Skip network]' \
            '--local[Skip daemon; resolve in-process]' \
            '--format[Output format]:format:(svg png)' \
            '--size[Pixel size]:size:' \
            '--theme[Prefer theme]:theme:(dark light)' \
            '--order[Stage type order override]:order:' \
            '1:query:_files'
          ;;
        sources)
          _arguments \
            '--json[Emit JSON]' \
            "1:subcommand:(${sources_cmds[*]})"
          ;;
        pack)
          _arguments \
            '--json[Emit JSON]' \
            '--path[Clone destination]:path:_files' \
            '--name[Pack name]:name:' \
            '--subdir[Pack root subdir]:subdir:' \
            '--ref[Git branch or tag]:ref:' \
            '--from-bundle[Install from tarball]:bundle:_files' \
            '--offline[Refuse network]' \
            "1:subcommand:(${pack_cmds[*]})" \
            '*:arg:'
          ;;
        daemon)
          _arguments '--socket[Unix socket path]:path:_files'
          ;;
        prefetch)
          _arguments '*:query:'
          ;;
        cache)
          _arguments "1:subcommand:(${cache_cmds[*]})"
          ;;
        override)
          _arguments \
            '--json[Emit JSON]' \
            "1:subcommand:(${override_cmds[*]})" \
            '*:arg:'
          ;;
        completion)
          _arguments "1:shell:(${shells[*]})"
          ;;
      esac
      ;;
  esac
}

_appicon "$@"
