{
  description = "Ryomarket development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            nativeBuildInputs = with pkgs; [
              go
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
