# bash completion for appicon
# shellcheck shell=bash
# Usage: source <(appicon completion bash)
#   or:  appicon completion bash > ~/.local/share/bash-completion/completions/appicon

_appicon() {
  local cur prev words cword
  if declare -F _init_completion >/dev/null 2>&1; then
    _init_completion || return
  else
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
  fi

  local cmds="resolve prefetch cache override sources pack daemon mcp completion man version help"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=($(compgen -W "${cmds}" -- "${cur}"))
    return
  fi

  local cmd="${COMP_WORDS[1]}"
  case "${cmd}" in
    resolve)
      case "${prev}" in
        --format) COMPREPLY=($(compgen -W "svg png" -- "${cur}")); return ;;
        --theme) COMPREPLY=($(compgen -W "dark light" -- "${cur}")); return ;;
        --size|--order) return ;;
      esac
      if [[ ${cur} == -* ]]; then
        COMPREPLY=($(compgen -W "--json --offline --local --format --size --theme --order --help" -- "${cur}"))
      fi
      ;;
    sources)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "list path" -- "${cur}"))
      elif [[ ${cur} == -* ]]; then
        COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
      fi
      ;;
    pack)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "list path add install update" -- "${cur}"))
      else
        case "${COMP_WORDS[2]}" in
          install)
            if [[ ${cur} == -* ]]; then
              COMPREPLY=($(compgen -W "--path --name --subdir --ref --from-bundle --offline --help" -- "${cur}"))
            else
              COMPREPLY=($(compgen -W "simple-icons dashboard-icons" -- "${cur}"))
            fi
            ;;
          update)
            COMPREPLY=($(compgen -W "simple-icons dashboard-icons --offline --help" -- "${cur}"))
            ;;
          list)
            COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
            ;;
        esac
      fi
      ;;
    daemon)
      case "${prev}" in
        --socket) return ;;
      esac
      if [[ ${cur} == -* ]]; then
        COMPREPLY=($(compgen -W "--socket --help" -- "${cur}"))
      fi
      ;;
    prefetch)
      if [[ ${cur} == -* ]]; then
        COMPREPLY=($(compgen -W "--help" -- "${cur}"))
      fi
      ;;
    cache)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "path clear stats prune" -- "${cur}"))
      fi
      ;;
    override)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "list get set rm path" -- "${cur}"))
        return
      fi
      if [[ ${cur} == -* ]]; then
        COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
      fi
      ;;
    completion)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
      fi
      ;;
    mcp|version|help)
      ;;
  esac
}

complete -F _appicon appicon
