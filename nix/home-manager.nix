# Home Manager module for appicon.
#
# Flake usage:
#   home-manager.users.you = { ... }: {
#     imports = [ inputs.appicon.homeManagerModules.default ];
#     programs.appicon.enable = true;
#     programs.appicon.daemon.enable = true; # optional user socket (Linux/systemd)
#     programs.appicon.configFormat = "yaml"; # optional: sources/overrides as YAML
#     programs.appicon.sources = {
#       sources = [
#         { type = "overrides"; }
#         { type = "xdg"; }
#         { type = "svgl"; }
#         { type = "logo-dev"; token_env = "LOGO_DEV_TOKEN"; }
#       ];
#     };
#     # BYOK: token_env names must resolve to secret *values*, never secret file paths.
#     # Prefer environmentFiles (KEY=value lines) with sops-nix templates, e.g.:
#     #   sops.templates."appicon.env".content = ''
#     #     LOGO_DEV_TOKEN=${config.sops.placeholder."logo-dev-token"}
#     #   '';
#     #   programs.appicon.environmentFiles = [ config.sops.templates."appicon.env".path ];
#     # Do NOT set LOGO_DEV_TOKEN = config.sops.secrets….path — that puts a path string
#     # into the env var that appicon reads as the API token.
#     # optional if not using the flake overlay:
#     # programs.appicon.package = inputs.appicon.packages.${pkgs.system}.appicon-bin;
#   };
#
# Completions: eval "$(appicon completion bash)" (or zsh/fish) — see README.
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.programs.appicon;
  format = if cfg.configFormat == "yaml" then "yaml" else "json";
  sourcesFile =
    if format == "yaml" then "appicon/sources.yaml" else "appicon/sources.json";
  overridesFile =
    if format == "yaml" then "appicon/overrides.yaml" else "appicon/overrides.json";
  encode =
    if format == "yaml" then lib.generators.toYAML { } else (v: builtins.toJSON v);
in
{
  options.programs.appicon = {
    enable = lib.mkEnableOption "appicon CLI (resolve desktop/brand icons to local paths)";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.appicon or (throw "pkgs.appicon missing — add appicon.overlays.default or set programs.appicon.package");
      defaultText = lib.literalExpression "pkgs.appicon";
      description = "appicon package to install (appicon / appicon-bin / appicon-git).";
    };

    configFormat = lib.mkOption {
      type = lib.types.enum [
        "json"
        "yaml"
      ];
      default = "json";
      description = "On-disk format for programs.appicon.sources / overrides (json or yaml).";
    };

    sources = lib.mkOption {
      type = lib.types.nullOr lib.types.attrs;
      default = null;
      description = ''
        Declarative sources config written to xdg config (same shape as sources.json).
        Example: { sources = [ { type = "xdg"; } { type = "svgl"; } ]; }
      '';
    };

    overrides = lib.mkOption {
      type = lib.types.nullOr (lib.types.attrsOf lib.types.str);
      default = null;
      description = "Declarative query→target remaps written to overrides.json/yaml.";
    };

    environment = lib.mkOption {
      type = lib.types.attrsOf lib.types.str;
      default = { };
      description = ''
        Session environment variables for BYOK tokens (token_env / secret_env *names* map here).
        Values must be the secret strings themselves — never a path to a secret file
        (do not assign sops secret paths here). Prefer environmentFiles + sops templates
        for real secrets; use this only for non-secret defaults or test tokens.
      '';
      example = lib.literalExpression ''
        {
          # Prefer environmentFiles for production secrets.
          # LOGO_DEV_TOKEN = "pk_test_…"; # local only
        }
      '';
    };

    environmentFiles = lib.mkOption {
      type = lib.types.listOf lib.types.path;
      default = [ ];
      description = ''
        systemd EnvironmentFile= paths for the optional daemon (KEY=value lines).
        Use with sops-nix / agenix templates so the file contains secret *values*.
        Interactive shells still need the same vars via your session (login env,
        or a matching template loaded by your shell); environmentFiles alone only
        affect the user systemd appicon service.
      '';
      example = lib.literalExpression ''
        [ config.sops.templates."appicon.env".path ]
      '';
    };

    daemon = {
      enable = lib.mkEnableOption ''
        optional user systemd socket daemon ($XDG_RUNTIME_DIR/appicon.sock).
        resolve dials the socket when present and falls back in-process.
        Linux/systemd Home Manager sessions only.
      '';
    };
  };

  config = lib.mkIf cfg.enable (
    lib.mkMerge [
      { home.packages = [ cfg.package ]; }

      (lib.mkIf (cfg.environment != { }) {
        home.sessionVariables = cfg.environment;
      })

      (lib.mkIf (cfg.sources != null) {
        xdg.configFile.${sourcesFile}.text = encode cfg.sources;
      })

      (lib.mkIf (cfg.overrides != null) {
        xdg.configFile.${overridesFile}.text = encode cfg.overrides;
      })

      (lib.mkIf cfg.daemon.enable {
        assertions = [
          {
            assertion = pkgs.stdenv.hostPlatform.isLinux;
            message = "programs.appicon.daemon.enable requires a Linux (systemd) Home Manager session";
          }
        ];

        systemd.user.sockets.appicon = {
          Unit = {
            Description = "appicon resolve socket";
            Documentation = "https://github.com/bolens/appicon";
          };
          Socket = {
            ListenStream = "%t/appicon.sock";
            SocketMode = "0600";
            DirectoryMode = "0700";
          };
          Install.WantedBy = [ "sockets.target" ];
        };

        systemd.user.services.appicon = {
          Unit = {
            Description = "appicon icon resolve daemon";
            Documentation = "https://github.com/bolens/appicon";
            Requires = [ "appicon.socket" ];
            After = [ "appicon.socket" ];
          };
          Service = {
            Type = "simple";
            # Absolute path so the user service does not depend on a login PATH.
            ExecStart = "${lib.getExe cfg.package} daemon";
            Restart = "on-failure";
            RestartSec = "1";
            # Non-secret defaults only — real BYOK secrets belong in EnvironmentFile.
            Environment = lib.mapAttrsToList (n: v: "${n}=${v}") cfg.environment;
            EnvironmentFile = cfg.environmentFiles;
          };
          Install = {
            Also = [ "appicon.socket" ];
            WantedBy = [ "default.target" ];
          };
        };
      })
    ]
  );
}
