self:
{ config, lib, pkgs, ... }:
let cfg = config.programs.ryomarket;
in {
  options.programs.ryomarket = {
    enable = lib.mkEnableOption "RyoMarket application discovery and management";
    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.ryomarket;
      description = "RyoMarket package to install.";
    };
  };
  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ cfg.package ];
  };
}
