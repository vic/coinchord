# NixOS module: services.coinchord.
# Runs CoinChord in every graphical user session (the mic belongs to the
# user's sound server) and gives the listed users the virtual keyboard.
self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.coinchord;
in
{
  options.services.coinchord = import ./options.nix self { inherit lib pkgs; } // {
    users = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      example = [ "alice" ];
      description = "Users allowed to send keyboard and mouse input through /dev/uinput.";
    };
  };

  config = lib.mkIf cfg.enable {
    hardware.uinput.enable = true;
    users.users = lib.genAttrs cfg.users (_: {
      extraGroups = [ "uinput" ];
    });
    environment.systemPackages = [ cfg.package ];
    systemd.user.services.coinchord = {
      description = "CoinChord acoustic chord keyboard";
      wantedBy = [ "graphical-session.target" ];
      partOf = [ "graphical-session.target" ];
      after = [
        "graphical-session.target"
        "pipewire.service"
      ];
      serviceConfig = {
        ExecStart = import ./command.nix cfg;
        Restart = "on-failure";
        RestartSec = 2;
      };
    };
  };
}
