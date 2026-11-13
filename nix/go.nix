# Keep source packages and development shells on the reviewed security patch.
# The maintained Darwin-compatible nixpkgs series still ships Go 1.26.10.
{ pkgs }:
pkgs.go_1_26.overrideAttrs (_: {
  version = "1.26.11";
  src = pkgs.fetchurl {
    url = "https://go.dev/dl/go1.26.11.src.tar.gz";
    hash = "sha256-1EEBsP8n7SpMda1GwXucdDWSCeLrLKfJ32y0/baOGWk=";
  };
})
