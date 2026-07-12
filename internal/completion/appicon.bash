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

  local cmds="resolve prefetch cache override sources pack status daemon mcp completion man version help"
  local stages="file overrides xdg svgl pack dir simple-icons dashboard-icons http-index github glyph"

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
        --order)
          local partial="${cur##*,}"
          local prefix=""
          if [[ "${cur}" == *,* ]]; then
            prefix="${cur%,*},"
          fi
          COMPREPLY=($(compgen -W "${stages}" -- "${partial}"))
          if [[ -n "${prefix}" ]]; then
            local i
            for i in "${!COMPREPLY[@]}"; do
              COMPREPLY[$i]="${prefix}${COMPREPLY[$i]}"
            done
          fi
          return
          ;;
        --size) return ;;
      esac
      if [[ ${cur} == -* ]]; then
        COMPREPLY=($(compgen -W "--json --explain --offline --local --format --size --theme --order --help" -- "${cur}"))
      else
        local qs
        qs="$(appicon __complete queries "${cur}" 2>/dev/null)"
        if [[ -n "${qs}" ]]; then
          COMPREPLY=($(compgen -W "${qs}" -- "${cur}"))
        fi
      fi
      ;;
    sources)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "list get set path" -- "${cur}"))
      else
        case "${prev}" in
          --file) COMPREPLY=($(compgen -f -- "${cur}")); return ;;
        esac
        if [[ ${cur} == -* ]]; then
          COMPREPLY=($(compgen -W "--json --file --help" -- "${cur}"))
        fi
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
      case "${prev}" in
        --order)
          local partial="${cur##*,}"
          local prefix=""
          if [[ "${cur}" == *,* ]]; then
            prefix="${cur%,*},"
          fi
          COMPREPLY=($(compgen -W "${stages}" -- "${partial}"))
          if [[ -n "${prefix}" ]]; then
            local i
            for i in "${!COMPREPLY[@]}"; do
              COMPREPLY[$i]="${prefix}${COMPREPLY[$i]}"
            done
          fi
          return
          ;;
      esac
      if [[ ${cur} == -* ]]; then
        COMPREPLY=($(compgen -W "--json --offline --from-desktop --theme --order --help" -- "${cur}"))
      else
        local qs
        qs="$(appicon __complete queries "${cur}" 2>/dev/null)"
        if [[ -n "${qs}" ]]; then
          COMPREPLY=($(compgen -W "${qs}" -- "${cur}"))
        fi
      fi
      ;;
    status)
      if [[ ${cur} == -* ]]; then
        COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
      fi
      ;;
    cache)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "path clear stats prune" -- "${cur}"))
      fi
      ;;
    override)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "list get set rm path suggest" -- "${cur}"))
        return
      fi
      case "${COMP_WORDS[2]}" in
        suggest)
          if [[ ${cur} == -* ]]; then
            COMPREPLY=($(compgen -W "--json --apply --from-misses --help" -- "${cur}"))
          else
            local qs
            qs="$(appicon __complete queries "${cur}" 2>/dev/null)"
            if [[ -n "${qs}" ]]; then
              COMPREPLY=($(compgen -W "${qs}" -- "${cur}"))
            fi
          fi
          ;;
        *)
          if [[ ${cur} == -* ]]; then
            COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
          else
            local qs
            qs="$(appicon __complete queries "${cur}" 2>/dev/null)"
            if [[ -n "${qs}" ]]; then
              COMPREPLY=($(compgen -W "${qs}" -- "${cur}"))
            fi
          fi
          ;;
      esac
      ;;
    completion)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
      fi
      ;;
    mcp|version|help|man)
      ;;
  esac
}

complete -F _appicon appicon
