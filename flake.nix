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
      version = "0.1.1";
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          appicon = pkgs.buildGoModule {
            pname = "appicon";
            inherit version;
            src = self;
            vendorHash = "sha256-USrxmmu8moHcfqZvtb/kS6rbcW4RaleCp0x6lkXfymY=";
            ldflags = [
              "-s"
              "-w"
              "-X github.com/bolens/appicon/internal/version.Version=v${version}"
            ];
            meta = with pkgs.lib; {
              description = "Resolve desktop and brand icons to local file paths";
              homepage = "https://github.com/bolens/appicon";
              license = licenses.mit;
              mainProgram = "appicon";
              platforms = platforms.linux ++ platforms.darwin;
            };
          };
        in
        {
          default = appicon;
          appicon = appicon;
        }
      );

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
      overlays.default = final: prev: {
        appicon = self.packages.${final.system}.appicon;
      };
    };
}
