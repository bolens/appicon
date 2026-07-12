#compdef appicon

_appicon() {
  local -a cmds cache_cmds override_cmds shells formats themes stages

  cmds=(
    'resolve:Resolve an icon query to a local path'
    'prefetch:Warm the icon cache for queries'
    'status:Show paths, order, cache, daemon, tools'
    'cache:Cache path/clear/stats/prune'
    'override:Manage overrides.json remaps'
    'sources:Manage sources.json / effective order'
    'pack:Manage local icon packs'
    'daemon:Run optional unix-socket resolve daemon'
    'mcp:Run stdio MCP server for agents'
    'completion:Print shell completion script'
    'man:Print man page (troff) to stdout'
    'version:Print version'
    'help:Show usage'
  )
  cache_cmds=(path clear stats prune)
  override_cmds=(list get set rm path suggest)
  sources_cmds=(list get set path)
  pack_cmds=(list path add install update)
  shells=(bash zsh fish)
  formats=(svg png)
  themes=(dark light)
  stages=(file overrides xdg svgl pack dir simple-icons dashboard-icons http-index github glyph)

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
            '--explain[Include tried stages / miss hint]' \
            '--offline[Skip network]' \
            '--local[Skip daemon; resolve in-process]' \
            '--format[Output format]:format:(svg png)' \
            '--size[Pixel size]:size:' \
            '--theme[Prefer theme]:theme:(dark light)' \
            "--order[Stage type order override]:order:(${stages[*]})" \
            '*:query:->queries'
          ;;
        sources)
          _arguments \
            '--json[Emit JSON]' \
            '--file[Read sources JSON from path]:file:_files' \
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
          _arguments \
            '--json[Emit JSON results]' \
            '--offline[Skip network]' \
            '--from-desktop[Derive queries from .desktop files]' \
            '--theme[Prefer theme]:theme:(dark light)' \
            "--order[Stage type order override]:order:(${stages[*]})" \
            '*:query:->queries'
          ;;
        status)
          _arguments '--json[Emit JSON]'
          ;;
        cache)
          _arguments "1:subcommand:(${cache_cmds[*]})"
          ;;
        override)
          _arguments \
            '--json[Emit JSON]' \
            '--apply[Apply first suggest candidate]' \
            '--from-misses[Suggest for recent misses]' \
            "1:subcommand:(${override_cmds[*]})" \
            '*:query:->queries'
          ;;
        completion)
          _arguments "1:shell:(${shells[*]})"
          ;;
        mcp|version|help|man)
          ;;
      esac
      if [[ $state == queries ]]; then
        local -a qs
        qs=(${(f)"$(appicon __complete queries ${words[-1]} 2>/dev/null)"})
        (( $#qs )) && _describe -t queries 'query' qs
      fi
      ;;
  esac
}

_appicon "$@"
