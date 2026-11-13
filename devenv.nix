{ pkgs, config, lib, ... }:
let
  goToolchain = import ./nix/go.nix { inherit pkgs; };
  developmentHome = pkgs.runCommand "development-home" { } ''
    mkdir -p "$out/env"
  '';
in
{
  name = "appicon";
  # Use existing Nix caches without changing daemon trust configuration.
  cachix.enable = false;
  # This repository has no background services or process-compose configuration.
  process.manager.implementation = "overmind";
  packages = with pkgs; [
    bashInteractive coreutils findutils gawk git gnugrep gnumake gnused
    goToolchain nodejs_24 python3 shellcheck openssh cacert curl gnutar gzip unzip which stdenv.cc
  ] ++ lib.optionals pkgs.stdenv.hostPlatform.isLinux [ pkgs.nss_wrapper ];
  # Only isolated signing-fixture children load this library for an unmapped UID.
  env.APPICON_TEST_NSS_WRAPPER = lib.optionalString pkgs.stdenv.hostPlatform.isLinux
    "${lib.getLib pkgs.nss_wrapper}/lib/libnss_wrapper.so";
  # Bind Go to its own standard library even when the host exports GOROOT.
  env.GOROOT = "${goToolchain}/share/go";
  env.GOTOOLCHAIN = "local";
  env.SSL_CERT_FILE = "${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt";
  scripts.repo-check.exec = "make check-fast check-nightly-smoke";
  enterTest = "repo-check";

  containers.shell = {
    name = "localhost/appicon-dev";
    version = "latest";
    # Mount source when running; never bake checkout files or local secrets in.
    copyToRoot = [ ];
    # Prepare the image's existing home; nothing is mounted here from the host.
    layers = lib.mkAfter [{
      copyToRoot = [ developmentHome ];
      perms = [{ path = developmentHome; regex = "/env"; mode = "1777"; }];
    }];
    entrypoint = [ (pkgs.writeShellScript "development-entrypoint" ''
      export PATH="${lib.makeBinPath config.packages}:$PATH"
      exec "$@"
    '') ];
    startupCommand = "bash";
  };
}
