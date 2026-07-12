#compdef appicon

_appicon() {
  local -a cmds cache_cmds shells formats themes

  cmds=(
    'resolve:Resolve an icon query to a local path'
    'prefetch:Warm the icon cache for queries'
    'cache:Cache path/clear/stats/prune'
    'mcp:Run stdio MCP server for agents'
    'completion:Print shell completion script'
    'man:Print man page (troff) to stdout'
    'version:Print version'
    'help:Show usage'
  )
  cache_cmds=(path clear stats prune)
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
            '--format[Output format]:format:(svg png)' \
            '--size[Pixel size]:size:' \
            '--theme[Prefer theme]:theme:(dark light)' \
            '1:query:_files'
          ;;
        prefetch)
          _arguments '*:query:'
          ;;
        cache)
          _arguments "1:subcommand:(${cache_cmds[*]})"
          ;;
        completion)
          _arguments "1:shell:(${shells[*]})"
          ;;
      esac
      ;;
  esac
}

_appicon "$@"
