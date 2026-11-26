# Options shared by the NixOS and home-manager modules.
self:
{ lib, pkgs, ... }:
{
  enable = lib.mkEnableOption "CoinChord, the acoustic chord keyboard, as a background service";

  package = lib.mkOption {
    type = lib.types.package;
    default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
    defaultText = lib.literalExpression "coinchord.packages.\${system}.default";
    description = "The CoinChord package.";
  };

  output = lib.mkOption {
    type = lib.types.enum [
      "on"
      "off"
    ];
    default = "off";
    description = ''
      Start with output on, so chords type, click and switch windows
      right away. You can switch it in the web interface at any time.
    '';
  };

  port = lib.mkOption {
    type = lib.types.port;
    default = 7317;
    description = "Port of the web interface on 127.0.0.1.";
  };

  extraArgs = lib.mkOption {
    type = lib.types.listOf lib.types.str;
    default = [ ];
    example = [
      "-record"
      "/home/me/coinchord-recordings"
    ];
    description = "More command-line flags for coinchord.";
  };
}
