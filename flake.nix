{
  description = "Ryomarket desktop application and development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    ryoku.url = "github:Ryoku-dev/ryoku/1d3feb1b8a6db9ee9ffdc15d0cb626828a5eed4a";
    ryotunes = {
      url = "github:Ryoku-dev/RyotunesNix/8b1b35aa258421ce8dffa3ca16127f7faef82f59";
      inputs.ryoku.follows = "ryoku";
      inputs.nixpkgs.follows = "ryoku/nixpkgs";
    };
  };

  outputs = { self, nixpkgs, ryoku, ryotunes, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;

      packagesFor = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          ryomarket = pkgs.callPackage ./nix/package.nix {
            appsFlakeRef = "path:${self.outPath}?narHash=${builtins.replaceStrings [ "+" "=" ] [ "%2B" "%3D" ] self.narHash}";
          };
        in
        {
          default = ryomarket;
          inherit ryomarket;
        } // nixpkgs.lib.optionalAttrs (system == "x86_64-linux")
          ((import ./nix/ryoku-apps.nix { inherit ryoku ryotunes system; }) // (import ./nix/flake-installers.nix {
            inherit pkgs;
            flakeRef = "path:${self.outPath}?narHash=${builtins.replaceStrings [ "+" "=" ] [ "%2B" "%3D" ] self.narHash}";
          })) // {
          ryomarket-nix-helper = ryomarket.nixHelper;
        }
      );
    in
    {
      packages = packagesFor;
      nixosModules.default = import ./nix/module.nix self;

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${packagesFor.${system}.default}/bin/ryomarket";
          meta.description = "Launch Ryomarket";
        };
      } // nixpkgs.lib.optionalAttrs (system == "x86_64-linux")
        (nixpkgs.lib.genAttrs [ "ryotunes" "ryomanager" "ryostore" "ryomotion" "rashin" "ryoport" "install-ryotunes" "install-ryomanager" "install-ryostore" "install-ryomotion" "install-rashin" "install-ryoport" ]
          (name: { type = "app"; program = nixpkgs.lib.getExe packagesFor.${system}.${name}; })));

      checks = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          package = packagesFor.${system}.default;
        in
        {
          ryomarket = package;
          package-layout = pkgs.runCommand "ryomarket-package-layout" { } ''
            test -x ${package}/bin/ryomarket
            test -x ${package}/bin/ryomarket-portable
            test ! -e ${package}/bin/ryomarket-nix
            test -x ${package.nixHelper}/bin/ryomarket-nix
            test -f ${package}/share/applications/dev.ryoku.Ryomarket.desktop
            test -f ${package}/share/metainfo/dev.ryoku.Ryomarket.metainfo.xml
            test -f ${package}/share/icons/hicolor/scalable/apps/dev.ryoku.Ryomarket.svg
            touch "$out"
          '';
        } // nixpkgs.lib.optionalAttrs (system == "x86_64-linux") {
          optional-app-layout = pkgs.runCommand "ryomarket-optional-app-layout" { } ''
            test -x ${packagesFor.${system}.ryotunes}/bin/ryotunes
            test -x ${packagesFor.${system}.ryomanager}/bin/ryomanager
            test -x ${packagesFor.${system}.ryomotion}/bin/ryomotion
            test -x ${packagesFor.${system}.ryostore}/bin/ryostore-ui
            test -x ${packagesFor.${system}.ryostore}/bin/ryostore
            test -f ${packagesFor.${system}.ryostore}/share/applications/ryostore.desktop
            test -x ${packagesFor.${system}.ryoport}/bin/ryoport
            test -x ${packagesFor.${system}.ryoport}/bin/ryovm
            test -x ${packagesFor.${system}.ryoport}/bin/ryossh
            test -f ${packagesFor.${system}.ryoport}/share/applications/ryoport.desktop
            test -x ${packagesFor.${system}.rashin}/bin/rashin-app
            test -x ${packagesFor.${system}.rashin}/bin/ryoku-rashin
            test -f ${packagesFor.${system}.rashin}/share/systemd/user/ryoku-rashin.service
            touch "$out"
          '';
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            nativeBuildInputs = with pkgs; [
              go
              python3
              gnumake
              gcc
              pkg-config
            ];

            buildInputs = with pkgs; [
              wayland

              libxkbcommon

              libx11
              libxcb
              libxcursor
              libxfixes

              libglvnd
              libffi

              vulkan-headers
            ];
          };
        }
      );
    };
}
