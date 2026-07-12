# AUR-parity packages for appicon.
#
# | Attr          | Like AUR        | Source                                      |
# |---------------|-----------------|---------------------------------------------|
# | appicon       | appicon         | buildGoModule from this flake (fixed ver)   |
# | appicon-bin   | appicon-bin     | GitHub release tarball (linux only)         |
# | appicon-git   | appicon-git     | buildGoModule from this flake (rev version) |
#
# Call as: import ./packages.nix { inherit pkgs self version vendorHash; }
{
  pkgs,
  self,
  version,
  vendorHash,
}:
let
  inherit (pkgs) lib;
  revSuffix =
    if self ? shortRev then self.shortRev
    else if self ? dirtyShortRev then self.dirtyShortRev
    else "dirty";

  mkGo =
    {
      pname,
      version,
      src,
    }:
    pkgs.buildGoModule {
      inherit pname version src vendorHash;
      ldflags = [
        "-s"
        "-w"
        "-X github.com/bolens/appicon/internal/version.Version=v${version}"
      ];
      # Nix sets HOME=/homeless-shelter; give tests a writable XDG tree.
      preCheck = ''
        export HOME="$TMPDIR/appicon-home"
        mkdir -p "$HOME"
        export XDG_CACHE_HOME="$HOME/.cache"
        export XDG_CONFIG_HOME="$HOME/.config"
        export XDG_DATA_HOME="$HOME/.local/share"
      '';
      postInstall = ''
        install -Dm644 contrib/systemd/appicon.socket \
          $out/lib/systemd/user/appicon.socket
        substitute contrib/systemd/appicon.service \
          $out/lib/systemd/user/appicon.service \
          --replace-fail 'ExecStart=appicon daemon' \
          "ExecStart=$out/bin/appicon daemon"
      '';
      meta = with lib; {
        description = "Resolve desktop and brand icons to local file paths";
        homepage = "https://github.com/bolens/appicon";
        license = licenses.mit;
        mainProgram = "appicon";
        platforms = platforms.linux ++ platforms.darwin;
      };
    };

  # Linux release asset hashes for v${version} (from SHA256SUMS → SRI).
  binHashes = {
    x86_64-linux = "sha256-QzKy4zvDnAlf0UVTRXF/U7zt3lpp1g/EmRZ0zirkOiU=";
    aarch64-linux = "sha256-F68XRxQ5itdy2sEWviuyFOQF2q/eg0ixNfPmLLr9zyc=";
  };

  goarchFor = system: {
    x86_64-linux = "amd64";
    aarch64-linux = "arm64";
  }.${system} or null;

  mkBin =
    system:
    let
      goarch = goarchFor system;
    in
    assert goarch != null;
    pkgs.stdenvNoCC.mkDerivation {
      pname = "appicon-bin";
      inherit version;
      src = pkgs.fetchurl {
        url = "https://github.com/bolens/appicon/releases/download/v${version}/appicon_v${version}_linux_${goarch}.tar.gz";
        hash = binHashes.${system};
      };
      sourceRoot = ".";
      dontConfigure = true;
      dontBuild = true;
      installPhase = ''
        runHook preInstall
        install -Dm755 appicon $out/bin/appicon
        if [ -f completions/appicon.bash ]; then
          install -Dm644 completions/appicon.bash \
            $out/share/bash-completion/completions/appicon
        fi
        if [ -f completions/appicon.zsh ]; then
          install -Dm644 completions/appicon.zsh \
            $out/share/zsh/site-functions/_appicon
        fi
        if [ -f completions/appicon.fish ]; then
          install -Dm644 completions/appicon.fish \
            $out/share/fish/vendor_completions.d/appicon.fish
        fi
        if [ -f man/man1/appicon.1 ]; then
          install -Dm644 man/man1/appicon.1 $out/share/man/man1/appicon.1
        fi
        if [ -f contrib/systemd/appicon.socket ]; then
          install -Dm644 contrib/systemd/appicon.socket \
            $out/lib/systemd/user/appicon.socket
        fi
        if [ -f contrib/systemd/appicon.service ]; then
          substitute contrib/systemd/appicon.service \
            $out/lib/systemd/user/appicon.service \
            --replace-fail 'ExecStart=appicon daemon' \
            "ExecStart=$out/bin/appicon daemon"
        fi
        runHook postInstall
      '';
      meta = with lib; {
        description = "Resolve desktop and brand icons to local file paths (prebuilt)";
        homepage = "https://github.com/bolens/appicon";
        license = licenses.mit;
        mainProgram = "appicon";
        platforms = [
          "x86_64-linux"
          "aarch64-linux"
        ];
        sourceProvenance = [ sourceTypes.binaryNativeCode ];
      };
    };
in
{
  appicon = mkGo {
    pname = "appicon";
    inherit version;
    src = self;
  };

  appicon-git = mkGo {
    pname = "appicon-git";
    version = "${version}-unstable-${revSuffix}";
    src = self;
  };

  appicon-bin =
    if builtins.hasAttr pkgs.stdenv.hostPlatform.system binHashes then
      mkBin pkgs.stdenv.hostPlatform.system
    else
      null;
}
