{
  lib,
  appsFlakeRef ? "",
  buildGoModule,
  makeWrapper,
  pkg-config,
  wayland,
  libxkbcommon,
  libx11,
  libxcb,
  libxcursor,
  libxfixes,
  libglvnd,
  libffi,
  vulkan-headers,
  vulkan-loader,
}:

buildGoModule rec {
  pname = "ryomarket";
  version = lib.removeSuffix "\n" (builtins.readFile ../VERSION);
  src = lib.cleanSource ../.;

  vendorHash = "sha256-NIAJ2wR9o1ei5TQlRLQcCUwzDHnQSd0bN4KhrptQ6p0=";

  ldflags = [ "-s" "-w" "-X main.version=${version}" ];

  outputs = [ "out" "nixHelper" ];
  subPackages = [
    "cmd/ryomarket"
    "cmd/ryomarket-portable"
    "cmd/ryomarket-nix"
  ];

  nativeBuildInputs = [
    pkg-config
    makeWrapper
  ];

  buildInputs = [
    wayland
    libxkbcommon
    libx11
    libxcb
    libxcursor
    libxfixes
    libglvnd
    libffi
    vulkan-headers
    vulkan-loader
  ];

  postInstall = ''
    install -Dm644 internal/ui/assets/FRAUNCES-OFL.txt "$out/share/licenses/ryomarket/FRAUNCES-OFL.txt"
    install -Dm644 internal/ui/assets/SPACE-GROTESK-OFL.txt \
      "$out/share/licenses/ryomarket/SPACE-GROTESK-OFL.txt"
    install -Dm644 packaging/dev.ryoku.Ryomarket.desktop \
      "$out/share/applications/dev.ryoku.Ryomarket.desktop"
    install -Dm644 packaging/dev.ryoku.Ryomarket.metainfo.xml \
      "$out/share/metainfo/dev.ryoku.Ryomarket.metainfo.xml"
    install -Dm644 assets/ryomarket.svg \
      "$out/share/icons/hicolor/scalable/apps/dev.ryoku.Ryomarket.svg"

    install -Dm755 "$out/bin/ryomarket-nix" \
      "$nixHelper/bin/ryomarket-nix"
    rm "$out/bin/ryomarket-nix"
  '';

  postFixup = ''
    wrapProgram "$out/bin/ryomarket" \
      --set RYOMARKET_APPS_FLAKE "${appsFlakeRef}" \
      --prefix LD_LIBRARY_PATH : "${lib.makeLibraryPath [
        wayland
        libxkbcommon
        libx11
        libxcb
        libxcursor
        libxfixes
        libglvnd
        vulkan-loader
      ]}"
  '';

  meta = {
    description = "Ryoku-native application marketplace";
    homepage = "https://github.com/aethctl/ryomarket";
    license = lib.licenses.gpl3Only;
    platforms = lib.platforms.linux;
    mainProgram = "ryomarket";
  };
}
