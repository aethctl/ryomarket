{ ryoku, ryotunes, system }:
let
  pkgs = import ryoku.inputs.nixpkgs { inherit system; };
  lib = pkgs.lib;
  src = ryoku.outPath;
  qml = ryoku.packages.${system}.ryoku-qml;
  config = pkgs.runCommand "ryoku-app-config" { } ''
    mkdir -p "$out/share/ryoku/config"
    cp -a ${src}/ryoku/hyprland "$out/share/ryoku/config/hypr"
    cp -a ${src}/ryoku/niri "$out/share/ryoku/config/niri"
  '';
  backend = import "${src}/nix/packages/ryoku-ryostore.nix" { inherit pkgs src; };
  portHelpers = import "${src}/nix/packages/ryoku-ryovm-helpers.nix" { inherit pkgs src; };
  rashinBackend = import "${src}/nix/packages/ryoku-rashin.nix" { inherit pkgs src; desktopData = config; };
  prowl = import "${src}/nix/packages/prowl-agent.nix" { inherit pkgs; };
  ryotunesBase = ryotunes.packages.${system}.ryotunes;
  ryotunesLauncher = pkgs.writeShellApplication {
    name = "ryotunes";
    runtimeInputs = [ ryotunesBase pkgs.coreutils pkgs.bash pkgs.zenity pkgs.xdg-utils pkgs.yt-dlp pkgs.mpv ];
    text = ''
      if ! timeout 2 ${ryotunesBase}/bin/ryotunes-cli hello >/dev/null 2>&1; then
        logs="''${XDG_STATE_HOME:-$HOME/.local/state}/ryotunes/logs"
        mkdir -p "$logs"
        nohup ${ryotunesBase}/bin/ryotunesd >"$logs/ryomarket-daemon.log" 2>&1 </dev/null &
        ready=0
        for _attempt in $(seq 1 100); do
          if timeout 1 ${ryotunesBase}/bin/ryotunes-cli hello >/dev/null 2>&1; then ready=1; break; fi
          sleep 0.1
        done
        if [[ "$ready" -eq 0 ]]; then echo "Ryotunes could not start its playback daemon. See $logs/ryomarket-daemon.log" >&2; exit 1; fi
      fi
      exec ${ryotunesBase}/bin/ryotunes "$@"
    '';
  };
  rashinLauncher = pkgs.writeShellApplication {
    name = "rashin-app";
    runtimeInputs = [ rashinBackend prowl pkgs.coreutils pkgs.curl pkgs.jq pkgs.git ];
    text = ''
      config="''${XDG_CONFIG_HOME:-$HOME/.config}/ryoku/rashin.json"
      port=3600
      if [[ -f "$config" ]]; then port=$(jq -r '.port // 3600' "$config"); fi
      if [[ ! "$port" =~ ^[0-9]+$ ]] || ((port < 1 || port > 65535)); then echo 'Invalid Rashin port' >&2; exit 1; fi
      child=""
      cleanup() { if [[ -n "$child" ]]; then kill "$child" 2>/dev/null || true; fi; }
      trap cleanup EXIT
      if ! curl --silent --fail --max-time 1 "http://127.0.0.1:$port/api/ping" >/dev/null; then
        logs="''${XDG_STATE_HOME:-$HOME/.local/state}/ryoku/rashin"
        mkdir -p "$logs"
        export RYOKU_RASHIN_SKILLS=${rashinBackend}/share/ryoku/skills
        export RYOKU_UPDATE_BACKEND=nix
        ${rashinBackend}/bin/ryoku-rashin serve >"$logs/ryomarket-daemon.log" 2>&1 &
        child=$!
        ready=0
        for _attempt in $(seq 1 100); do
          if curl --silent --fail --max-time 1 "http://127.0.0.1:$port/api/ping" >/dev/null; then ready=1; break; fi
          if ! kill -0 "$child" 2>/dev/null; then break; fi
          sleep 0.1
        done
        if [[ "$ready" -eq 0 ]]; then echo "Rashin could not start. See $logs/ryomarket-daemon.log" >&2; exit 1; fi
      fi
      ${rashinUI}/bin/rashin-app "$@"
    '';
  };
  qmlImports = lib.makeSearchPath "lib/qt-6/qml" [
    qml pkgs.qt6.qtdeclarative pkgs.qt6.qtmultimedia pkgs.qt6.qtwayland
    pkgs.qt6.qt5compat pkgs.qt6.qtsvg pkgs.qt6.qtimageformats
    pkgs.kdePackages.kirigami.unwrapped
  ];
  mkQmlApp = { name, title, appDir, runtimeInputs, extraInstall ? "", launcher ? name }:
    pkgs.stdenvNoCC.mkDerivation {
      pname = name;
      version = "unstable";
      dontUnpack = true;
      nativeBuildInputs = [ pkgs.makeWrapper ];
      installPhase = ''
        mkdir -p "$out/bin" "$out/share/${name}" "$out/share/applications" "$out/share/icons/hicolor/scalable/apps"
        cp -a ${src}/${appDir}/quickshell/. "$out/share/${name}/"
        makeWrapper ${pkgs.quickshell}/bin/qs "$out/bin/${launcher}" \
          --add-flags "-p $out/share/${name}" \
          --prefix QML_IMPORT_PATH : "${qmlImports}" \
          --prefix QML2_IMPORT_PATH : "${qmlImports}" \
          --prefix PATH : "$out/bin:${lib.makeBinPath runtimeInputs}"
        cp ${src}/${appDir}/quickshell/logo.svg "$out/share/icons/hicolor/scalable/apps/${name}.svg"
        cat > "$out/share/applications/${name}.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=${title}
Exec=$out/bin/${launcher}
Icon=${name}
Terminal=false
Categories=System;Utility;
DESKTOP
        ${extraInstall}
      '';
      meta = { description = title; license = lib.licenses.gpl3Plus; platforms = [ "x86_64-linux" ]; mainProgram = launcher; };
    };
  rashinUI = pkgs.stdenv.mkDerivation {
    pname = "rashin-app";
    version = "1.0.0";
    src = src + "/ryoku/rashin/app";
    nativeBuildInputs = [ pkgs.cmake pkgs.pkg-config pkgs.wrapGAppsHook3 ];
    buildInputs = [ pkgs.gtk3 pkgs.webkitgtk_4_1 ];
    postInstall = ''
      install -Dm644 ${src}/ryoku/rashin/app/logo.svg "$out/share/icons/hicolor/scalable/apps/rashin-app.svg"
      install -Dm644 ${src}/ryoku/rashin/app/rashin-app.desktop "$out/share/applications/rashin-app.desktop"
      substituteInPlace "$out/share/applications/rashin-app.desktop" --replace-fail "Exec=rashin-app" "Exec=$out/bin/rashin-app"
    '';
  };
in {
  ryotunes = pkgs.symlinkJoin {
    name = "ryotunes-standalone-${ryotunesBase.version or "1.2.3"}";
    paths = [ ryotunesBase ];
    postBuild = ''
      rm "$out/bin/ryotunes"
      ln -s ${ryotunesLauncher}/bin/ryotunes "$out/bin/ryotunes"
      rm "$out/share/applications/ryotunes.desktop"
      cp ${ryotunesBase}/share/applications/ryotunes.desktop "$out/share/applications/ryotunes.desktop"
      chmod u+w "$out/share/applications/ryotunes.desktop"
      substituteInPlace "$out/share/applications/ryotunes.desktop" --replace-fail 'Exec=ryotunes' "Exec=$out/bin/ryotunes"
    '';
    meta = ryotunesBase.meta // { mainProgram = "ryotunes"; };
  };
  ryotunes-soloist-setup = ryotunes.packages.${system}.soloist-setup;
  ryomanager = import "${src}/nix/packages/ryomanager.nix" { inherit pkgs; ryomanagerSrc = ryoku.inputs.ryomanagerSrc; };
  ryomotion = import "${src}/nix/packages/ryoku-ryomotion.nix" { inherit pkgs; };
  rashin = pkgs.symlinkJoin {
    name = "rashin-unstable";
    paths = [ rashinBackend rashinUI prowl ];
    postBuild = ''
      rm "$out/bin/rashin-app"
      ln -s ${rashinLauncher}/bin/rashin-app "$out/bin/rashin-app"
      rm "$out/share/applications/rashin-app.desktop"
      cp ${rashinUI}/share/applications/rashin-app.desktop "$out/share/applications/rashin-app.desktop"
      chmod u+w "$out/share/applications/rashin-app.desktop"
      substituteInPlace "$out/share/applications/rashin-app.desktop" --replace-fail ${rashinUI}/bin/rashin-app "$out/bin/rashin-app"
      mkdir -p "$out/share/systemd/user"
      cp ${src}/ryoku/rashin/systemd/ryoku-rashin.service "$out/share/systemd/user/ryoku-rashin.service"
      chmod u+w "$out/share/systemd/user/ryoku-rashin.service"
      substituteInPlace "$out/share/systemd/user/ryoku-rashin.service" \
        --replace-fail /usr/bin/ryoku-rashin ${rashinBackend}/bin/ryoku-rashin
    '';
    meta = { mainProgram = "rashin-app"; platforms = [ "x86_64-linux" ]; };
  };
  ryostore = mkQmlApp {
    name = "ryostore"; title = "RyoStore"; appDir = "ryoku/apps/ryostore"; launcher = "ryostore-ui";
    runtimeInputs = [ backend pkgs.quickshell pkgs.bash pkgs.coreutils pkgs.curl pkgs.jq pkgs.git pkgs.util-linux pkgs.findutils pkgs.gnugrep pkgs.gnused pkgs.xdg-utils pkgs.rsync pkgs.unzip pkgs.gnutar pkgs.zstd ];
    extraInstall = ''ln -s ${backend}/bin/ryostore "$out/bin/ryostore"'';
  };
  ryoport = mkQmlApp {
    name = "ryoport"; title = "Ryoport"; appDir = "ryoku/apps/ryovm";
    runtimeInputs = [ portHelpers pkgs.bash pkgs.coreutils pkgs.curl pkgs.jq pkgs.gnugrep pkgs.gnused pkgs.gawk pkgs.procps pkgs.util-linux pkgs.openssh pkgs.quickemu pkgs.qemu pkgs.virt-viewer pkgs.cdrtools ];
    extraInstall = ''
      install -Dm755 ${src}/ryoku/apps/ryovm/bin/ryovm "$out/bin/ryovm"
      wrapProgram "$out/bin/ryovm" --prefix PATH : "${lib.makeBinPath [ portHelpers pkgs.bash pkgs.coreutils pkgs.curl pkgs.jq pkgs.gnugrep pkgs.gnused pkgs.gawk pkgs.procps pkgs.util-linux pkgs.openssh pkgs.quickemu pkgs.qemu pkgs.virt-viewer pkgs.cdrtools ]}"
      for helper in ryovm-fetch ryovm-mon ryossh; do ln -s ${portHelpers}/bin/$helper "$out/bin/$helper"; done
    '';
  };
}
