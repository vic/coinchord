# home-manager module: services.coinchord.
# Output on Linux needs write access to /dev/uinput (the NixOS module sets
# that up; elsewhere join the group that owns /dev/uinput).
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
  options.services.coinchord = import ./options.nix self { inherit lib pkgs; };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];
    systemd.user.services.coinchord = {
      Unit = {
        Description = "CoinChord acoustic chord keyboard";
        PartOf = [ "graphical-session.target" ];
        After = [
          "graphical-session.target"
          "pipewire.service"
        ];
      };
      Service = {
        ExecStart = import ./command.nix cfg;
        Restart = "on-failure";
        RestartSec = 2;
      };
      Install.WantedBy = [ "graphical-session.target" ];
    };
  };
}
