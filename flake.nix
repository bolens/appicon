{
  description = "appicon — resolve desktop and brand icons to local file paths";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      version = "0.2.2";
      vendorHash = "sha256-OLRfiW2HGvALv0v05vUchIF3353FuRyyMgC+IS62e9Y=";
      packagesFor =
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          pack = import ./nix/packages.nix {
            inherit pkgs self version vendorHash;
          };
        in
        {
          default = pack.appicon;
          appicon = pack.appicon;
          appicon-git = pack.appicon-git;
        }
        // nixpkgs.lib.optionalAttrs (pack.appicon-bin != null) {
          appicon-bin = pack.appicon-bin;
        };
    in
    {
      packages = forAllSystems packagesFor;

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/appicon";
        };
        appicon = self.apps.${system}.default;
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.golangci-lint
              self.packages.${system}.appicon
            ];
          };
        }
      );

      homeManagerModules.default = import ./nix/home-manager.nix;

      overlays.default =
        final: prev:
        {
          appicon = self.packages.${final.system}.appicon;
          appicon-git = self.packages.${final.system}.appicon-git;
        }
        // nixpkgs.lib.optionalAttrs (builtins.hasAttr "appicon-bin" self.packages.${final.system}) {
          appicon-bin = self.packages.${final.system}.appicon-bin;
        };
    };
}
