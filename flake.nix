{
  description = "CoinChord — type and drive your computer with two coins and your laptop mic";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAll = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
      version = "0.4.0";
    in
    {
      packages = forAll (pkgs: rec {
        # The web interface, compiled offline from the pinned Elm packages.
        ui = pkgs.stdenv.mkDerivation {
          pname = "coinchord-ui";
          inherit version;
          src = ./web;
          nativeBuildInputs = [ pkgs.elmPackages.elm ];
          buildPhase =
            pkgs.elmPackages.fetchElmDeps {
              elmPackages = import ./web/elm-srcs.nix;
              elmVersion = "0.19.2";
              registryDat = ./web/registry.dat;
            }
            + ''
              elm make src/Main.elm --optimize --output=elm.js
            '';
          installPhase = "install -Dm644 elm.js $out/elm.js";
        };

        coinchord = pkgs.buildGoModule {
          pname = "coinchord";
          inherit version;
          src = ./.;
          vendorHash = "sha256-Z2RF4B2VWCnF45vdOdStVA6h2x8IxZJs8zpWDBUzYAo=";
          subPackages = [ "cmd/coinchord" ];
          env.CGO_ENABLED = "0";
          preBuild = "cp ${ui}/elm.js internal/server/dist/elm.js";
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postInstall = ''
            wrapProgram $out/bin/coinchord --prefix PATH : ${
              pkgs.lib.makeBinPath (
                [ pkgs.sox ]
                ++ pkgs.lib.optionals pkgs.stdenv.hostPlatform.isLinux [
                  pkgs.alsa-utils
                  pkgs.wireplumber
                  pkgs.xdg-utils
                ]
              )
            }
          '';
          meta.mainProgram = "coinchord";
        };

        default = coinchord;
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.elmPackages.elm
            pkgs.elm2nix
            pkgs.sox
          ]
          ++ pkgs.lib.optionals pkgs.stdenv.hostPlatform.isLinux [ pkgs.alsa-utils ];
        };
      });

      nixosModules.default = import ./nix/nixos.nix self;
      homeManagerModules.default = import ./nix/home-manager.nix self;
    };
}
