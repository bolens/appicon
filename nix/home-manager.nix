# Home Manager module for appicon.
#
# Flake usage:
#   home-manager.users.you = { ... }: {
#     imports = [ inputs.appicon.homeManagerModules.default ];
#     programs.appicon.enable = true;
#     programs.appicon.daemon.enable = true; # optional user socket
#     # optional if not using the flake overlay:
#     # programs.appicon.package = inputs.appicon.packages.${pkgs.system}.appicon-bin;
#     # programs.appicon.package = inputs.appicon.packages.${pkgs.system}.appicon-git;
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
