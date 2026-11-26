# The service command line, shared by both modules.
cfg:
builtins.concatStringsSep " " (
  [
    "${cfg.package}/bin/coinchord"
    "-open=false"
    "-output=${cfg.output}"
    "-listen=127.0.0.1:${toString cfg.port}"
  ]
  ++ cfg.extraArgs
)
