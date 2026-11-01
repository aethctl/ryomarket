{ pkgs, flakeRef }:
let
  apps = {
    ryotunes = { title = "Ryotunes"; program = "ryotunes"; };
    ryomanager = { title = "RyoManager"; program = "ryomanager"; };
    ryostore = { title = "RyoStore"; program = "ryostore-ui"; };
    ryomotion = { title = "RyoMotion"; program = "ryomotion"; };
    rashin = { title = "Rashin"; program = "rashin-app"; };
    ryoport = { title = "Ryoport"; program = "ryoport"; };
  };
in pkgs.lib.mapAttrs' (name: app: pkgs.lib.nameValuePair "install-${name}" (pkgs.writeShellApplication {
  name = "ryomarket-install-${name}";
  runtimeInputs = [ pkgs.nix pkgs.coreutils pkgs.util-linux pkgs.desktop-file-utils ];
  text = ''
    export RYOMARKET_APP_NAME=${pkgs.lib.escapeShellArg name}
    export RYOMARKET_APP_TITLE=${pkgs.lib.escapeShellArg app.title}
    export RYOMARKET_APP_PROGRAM=${pkgs.lib.escapeShellArg app.program}
    export RYOMARKET_APP_FLAKE=${pkgs.lib.escapeShellArg flakeRef}
    export RYOMARKET_APP_ICON=${builtins.path { path = ../internal/ui/assets + "/${name}.png"; name = "${name}.png"; }}
    ${builtins.readFile ./install-app.sh}
  '';
})) apps
