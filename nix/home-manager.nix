# Home Manager module for appicon.
#
# Flake usage:
#   home-manager.users.you = { ... }: {
#     imports = [ inputs.appicon.homeManagerModules.default ];
#     programs.appicon.enable = true;
#     programs.appicon.daemon.enable = true; # optional user socket
#     programs.appicon.configFormat = "yaml"; # optional: sources/overrides as YAML
#     programs.appicon.sources = {
#       sources = [
#         { type = "overrides"; }
#         { type = "xdg"; }
#         { type = "svgl"; }
#         { type = "logo-dev"; token_env = "LOGO_DEV_TOKEN"; }
#       ];
#     };
#     programs.appicon.environment = {
#       # Prefer sops/agenix for the actual secret values.
#       LOGO_DEV_TOKEN = config.sops.secrets.logo-dev-token.path; # or sessionVariables with plaintext (not recommended)
#     };
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
        Session environment variables for BYOK tokens (token_env names).
        Prefer sops-nix / agenix for secret values — do not commit API keys in Nix.
      '';
      example = lib.literalExpression ''
        {
          LOGO_DEV_TOKEN = "pk_…"; # better: wire via sops
          GITHUB_TOKEN = "ghp_…";
        }
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
            # Propagate BYOK env into the daemon if set via sessionVariables.
            Environment = lib.mapAttrsToList (n: v: "${n}=${v}") cfg.environment;
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
