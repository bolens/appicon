# Home Manager module for appicon.
#
# Flake usage:
#   home-manager.users.you = { ... }: {
#     imports = [ inputs.appicon.homeManagerModules.default ];
#     programs.appicon.enable = true;
#     # optional if not using the flake overlay:
#     # programs.appicon.package = inputs.appicon.packages.${pkgs.system}.default;
#   };
#
# Completions: eval "$(appicon completion bash)" (or zsh/fish) — see README.
{ config, lib, pkgs, ... }:
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
      description = "appicon package to install.";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];
  };
}
