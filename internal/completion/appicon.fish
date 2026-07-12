# fish completion for appicon
# Usage: appicon completion fish | source
#   or:  appicon completion fish > ~/.config/fish/completions/appicon.fish

complete -c appicon -f

complete -c appicon -n __fish_use_subcommand -a resolve -d 'Resolve an icon query to a local path'
complete -c appicon -n __fish_use_subcommand -a prefetch -d 'Warm the icon cache for queries'
complete -c appicon -n __fish_use_subcommand -a status -d 'Show paths, order, cache, daemon, tools'
complete -c appicon -n __fish_use_subcommand -a cache -d 'Cache path/clear/stats/prune'
complete -c appicon -n __fish_use_subcommand -a override -d 'Manage overrides.json remaps'
complete -c appicon -n __fish_use_subcommand -a sources -d 'Manage sources.json / effective order'
complete -c appicon -n __fish_use_subcommand -a pack -d 'Manage local icon packs'
complete -c appicon -n __fish_use_subcommand -a daemon -d 'Run optional unix-socket resolve daemon'
complete -c appicon -n __fish_use_subcommand -a mcp -d 'Run stdio MCP server for agents'
complete -c appicon -n __fish_use_subcommand -a completion -d 'Print shell completion script'
complete -c appicon -n __fish_use_subcommand -a man -d 'Print man page (troff) to stdout'
complete -c appicon -n __fish_use_subcommand -a version -d 'Print version'
complete -c appicon -n __fish_use_subcommand -a help -d 'Show usage'

set -l __appicon_stages file overrides xdg svgl pack dir simple-icons dashboard-icons http-index github glyph

complete -c appicon -n '__fish_seen_subcommand_from resolve' -l json -d 'Emit JSON result'
complete -c appicon -n '__fish_seen_subcommand_from resolve' -l explain -d 'Include tried stages / miss hint'
complete -c appicon -n '__fish_seen_subcommand_from resolve' -l offline -d 'Skip network'
complete -c appicon -n '__fish_seen_subcommand_from resolve' -l local -d 'Skip daemon; resolve in-process'
complete -c appicon -n '__fish_seen_subcommand_from resolve' -l format -xa 'svg png'
complete -c appicon -n '__fish_seen_subcommand_from resolve' -l size -r -d 'Pixel size'
complete -c appicon -n '__fish_seen_subcommand_from resolve' -l theme -xa 'dark light'
complete -c appicon -n '__fish_seen_subcommand_from resolve' -l order -xa "$__appicon_stages" -d 'Stage type order override'

complete -c appicon -n '__fish_seen_subcommand_from sources; and not __fish_seen_subcommand_from list get set path' -a list -d 'List effective order'
complete -c appicon -n '__fish_seen_subcommand_from sources; and not __fish_seen_subcommand_from list get set path' -a get -d 'Read sources.json'
complete -c appicon -n '__fish_seen_subcommand_from sources; and not __fish_seen_subcommand_from list get set path' -a set -d 'Overwrite sources.json'
complete -c appicon -n '__fish_seen_subcommand_from sources; and not __fish_seen_subcommand_from list get set path' -a path -d 'Print sources.json path'
complete -c appicon -n '__fish_seen_subcommand_from sources' -l json -d 'Emit JSON'
complete -c appicon -n '__fish_seen_subcommand_from sources' -l file -r -F -d 'Read sources JSON from path'

complete -c appicon -n '__fish_seen_subcommand_from pack; and not __fish_seen_subcommand_from list path add install update' -a list -d 'List packs'
complete -c appicon -n '__fish_seen_subcommand_from pack; and not __fish_seen_subcommand_from list path add install update' -a path -d 'Print packs root'
complete -c appicon -n '__fish_seen_subcommand_from pack; and not __fish_seen_subcommand_from list path add install update' -a add -d 'Register a pack directory'
complete -c appicon -n '__fish_seen_subcommand_from pack; and not __fish_seen_subcommand_from list path add install update' -a install -d 'Clone a recipe pack'
complete -c appicon -n '__fish_seen_subcommand_from pack; and not __fish_seen_subcommand_from list path add install update' -a update -d 'Refresh cloned packs'
complete -c appicon -n '__fish_seen_subcommand_from pack' -l json -d 'Emit JSON'
complete -c appicon -n '__fish_seen_subcommand_from pack' -l path -r -d 'Clone destination'
complete -c appicon -n '__fish_seen_subcommand_from pack' -l name -r -d 'Pack name'
complete -c appicon -n '__fish_seen_subcommand_from pack' -l subdir -r -d 'Pack root subdir'
complete -c appicon -n '__fish_seen_subcommand_from pack' -l ref -r -d 'Git branch or tag'
complete -c appicon -n '__fish_seen_subcommand_from pack' -l from-bundle -r -d 'Install from tarball'
complete -c appicon -n '__fish_seen_subcommand_from pack' -l offline -d 'Refuse network'

complete -c appicon -n '__fish_seen_subcommand_from daemon' -l socket -r -d 'Unix socket path'

complete -c appicon -n '__fish_seen_subcommand_from prefetch' -l json -d 'Emit JSON results'
complete -c appicon -n '__fish_seen_subcommand_from prefetch' -l offline -d 'Skip network'
complete -c appicon -n '__fish_seen_subcommand_from prefetch' -l order -xa "$__appicon_stages" -d 'Stage type order override'

complete -c appicon -n '__fish_seen_subcommand_from status' -l json -d 'Emit JSON'

complete -c appicon -n '__fish_seen_subcommand_from cache; and not __fish_seen_subcommand_from path clear stats prune' -a path -d 'Print cache directory'
complete -c appicon -n '__fish_seen_subcommand_from cache; and not __fish_seen_subcommand_from path clear stats prune' -a clear -d 'Delete cache'
complete -c appicon -n '__fish_seen_subcommand_from cache; and not __fish_seen_subcommand_from path clear stats prune' -a stats -d 'Cache stats'
complete -c appicon -n '__fish_seen_subcommand_from cache; and not __fish_seen_subcommand_from path clear stats prune' -a prune -d 'Prune stale entries'
complete -c appicon -n '__fish_seen_subcommand_from override; and not __fish_seen_subcommand_from list get set rm path' -a list -d 'List remaps'
complete -c appicon -n '__fish_seen_subcommand_from override; and not __fish_seen_subcommand_from list get set rm path' -a get -d 'Get remap'
complete -c appicon -n '__fish_seen_subcommand_from override; and not __fish_seen_subcommand_from list get set rm path' -a set -d 'Set remap'
complete -c appicon -n '__fish_seen_subcommand_from override; and not __fish_seen_subcommand_from list get set rm path' -a rm -d 'Remove remap'
complete -c appicon -n '__fish_seen_subcommand_from override; and not __fish_seen_subcommand_from list get set rm path' -a path -d 'Print overrides.json path'
complete -c appicon -n '__fish_seen_subcommand_from override' -l json -d 'Emit JSON'

complete -c appicon -n '__fish_seen_subcommand_from completion' -a bash -d 'Bash completion'
complete -c appicon -n '__fish_seen_subcommand_from completion' -a zsh -d 'Zsh completion'
complete -c appicon -n '__fish_seen_subcommand_from completion' -a fish -d 'Fish completion'
