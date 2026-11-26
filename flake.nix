{
  description = "CoinChord — Acoustic Word-Chording Keyboard (Go)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
    in
    {
      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go alsa-utils sox ];
        };
      });

      packages = forAll (pkgs: {
        default = pkgs.buildGoModule {
          pname = "coinchord";
          version = "0.3.0";
          src = ./.;
          vendorHash = "sha256-IvEt549O9UgqUgAwODGLpzw5HVV+rVLqPZhsXyeGBVE=";
          subPackages = [ "cmd/coinchord" ];
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postInstall = ''
            wrapProgram $out/bin/coinchord --prefix PATH : ${pkgs.lib.makeBinPath (with pkgs; [ alsa-utils sox ])}
          '';
        };
      });
    };
}

