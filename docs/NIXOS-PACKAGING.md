# Nix packaging

RyoMarket is packaged with `buildGoModule`, a fixed dependency hash and the required Wayland/X11 graphics libraries. A desktop entry, icon, AppStream metadata and font licenses are installed with the client.

## Public outputs

- `packages.<system>.default` and `.ryomarket`: the desktop client.
- `packages.<system>.ryomarket-nix-helper`: the separate legacy privileged helper, not included in the default package or NixOS module.
- `nixosModules.default`: `programs.ryomarket.enable` and an optional package override.
- On x86_64-linux: individual `ryotunes`, `ryomanager`, `ryostore`, `ryomotion`, `rashin` and `ryoport` packages, direct launch apps and `install-<name>` managed installer apps.
- `ryotunes-soloist-setup`: optional upstream Spotify backend setup.

The optional applications keep their own locked Ryoku/Qt package universe. Do not force those inputs to follow the host's nixpkgs. The marketplace client can be updated without pulling the application closures into the base desktop.

## Direct system declaration

With `ryomarket` in your flake's outputs arguments:

```nix
environment.systemPackages = [
  ryomarket.packages.${pkgs.stdenv.hostPlatform.system}.ryomarket
];
```

For optional applications, use the corresponding package output in the module that owns your app choices. Keep the flake locked. RyoMarket's configuration guidance does not mutate configuration or trigger a rebuild.

## Managed flake installs

`nix run github:aethctl/ryomarket#install-<name>` creates a separate profile generation under `$XDG_STATE_HOME/ryomarket/flake-apps/<name>`. It builds before publishing the active generation, then registers desktop and command links. Existing unrelated files are never overwritten. `-- --remove` removes only the managed installation, retaining the application's own data.

The packaged client retains an immutable source snapshot and hash. Its installer plans therefore use the exact recipes shipped with that client rather than following a mutable branch. Reinstall/update can select the source provided by a newer marketplace build. No system rebuild is involved.

## Validation

```sh
nix develop -c go test ./...
nix develop -c go vet ./...
nix develop -c python3 scripts/test-flake-installer.py
nix build .#checks.x86_64-linux.package-layout
```

CI evaluates flake outputs, runs the Go and installer tests, and builds the client package layout. Individual application builds and fresh-profile runtime tests are needed when changing app recipes. Model authentication, Spotify authentication, display/GPU behavior and KVM setup need runtime verification on suitable systems.
