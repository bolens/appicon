{
  description = "appicon — resolve desktop and brand icons to local file paths";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = "0.1.0";
        appicon = pkgs.buildGoModule {
          pname = "appicon";
          inherit version;
          src = ./.;
          # Bump after go.mod/go.sum changes:
          #   nix build .# 2>&1 | sed -n 's/.*got: *//p'
          vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
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
        packages.default = appicon;
        packages.appicon = appicon;
        apps.default = {
          type = "app";
          program = "${appicon}/bin/appicon";
        };
        apps.appicon = self.apps.${system}.default;
        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.golangci-lint
            appicon
          ];
        };
      }
    )
    // {
      homeManagerModules.default = import ./nix/home-manager.nix;
      overlays.default = final: prev: {
        appicon = self.packages.${final.system}.appicon;
      };
    };
}
