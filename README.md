# Ryomarket

Ryomarket is a Ryoku-native Linux software marketplace built in Go. It presents one unified catalogue across Ryoku distros, then resolves the installation methods that are valid for the detected host.

Supported distro families:

- Fedora
- Arch Linux
- CachyOS
- Void Linux
- NixOS

Supported provider types:

- DNF/RPM repositories
- pacman repositories
- XBPS repositories
- Nixpkgs profiles on every supported distro
- guarded declarative NixOS configuration
- Flatpak/Flathub
- managed AppImages
- curated upstream tar.gz releases

Ryomarket automatically detects the host distro. Other distro catalogues can be browsed, but install and remove actions stay locked when browsing a different distro.

## Current application flow

The Go/Gio desktop client includes Discover, Installed and Updates views, distro switching, search, category filtering, unified application details, provider selection, transaction previews and explicit confirmation before changes are made.

Flatpak-backed entries can enrich their detail page from Flathub AppStream metadata. Native, portable and Nix variants remain part of the same application identity rather than appearing as duplicate listings.

On Fedora, Arch/CachyOS and Void, Ryomarket can bootstrap Flatpak through the native package manager when it is missing. That prerequisite appears in the transaction preview before confirmation. NixOS keeps Flatpak setup explicit because enabling it is a system configuration choice.

Nixpkgs is also a universal application provider. Ryoku installations on Fedora, Arch/CachyOS and Void should ship the distro-packaged Nix runtime so users can choose Nixpkgs alongside the host repository, Flatpak and AppImage. `scripts/install-nix-runtime.sh` provides the same bootstrap for existing systems. NixOS uses its existing Nix installation. The Discover catalogue is not split by distro: cards carry availability markers such as `UNIVERSAL`, `FEDORA`, `ARCH`, `VOID`, `NIX` and `UNIVERSAL*`, where the asterisk means the Nix runtime is required on non-NixOS hosts.

The catalogue keeps up to 10,000 app-like packages from the active native repository and up to 60,000 filtered Nixpkgs entries. The full index remains searchable while Discover renders a bounded result set for responsive browsing.

The bundled seed catalog is intentionally curated. It proves the identity and provider model without guessing package mappings. The catalog is designed to grow independently from the UI.

## Build

Ryomarket targets Go 1.23 or newer and Gio.

```bash
go mod download
make test
make build
./build/ryomarket
```

Gio requires the normal Linux Wayland/X11, xkbcommon and graphics development libraries for a desktop build. The upstream Gio Linux guide lists Wayland, X11, xkbcommon, GLES/EGL and Xcursor development packages, with Vulkan support recommended.

For a user-local install:

```bash
./scripts/install-local.sh
```

This defaults to `~/.local`. Set `PREFIX` to choose another prefix. The user-local installer intentionally does not install the privileged `ryomarket-nix` helper.

## NixOS declarative provider

Declarative installs are deliberately opt-in. Add the managed module to `/etc/nixos/configuration.nix`:

```nix
imports = [
  ./ryomarket.nix
];
```

Then create `/etc/nixos/ryomarket.nix` from `packaging/nixos/ryomarket.nix.example` and rebuild once. The `ryomarket-nix` helper must be installed system-wide as a root-owned executable. Ryomarket rejects user-owned or group/world-writable copies before requesting elevation. Systems using a different configuration layout can still use Nix profile or Flatpak providers.

## Data locations

Ryomarket follows XDG paths:

- state and transaction history: `$XDG_STATE_HOME/ryomarket`
- managed AppImages: `$XDG_DATA_HOME/ryomarket/appimages`
- managed upstream applications: `$XDG_DATA_HOME/ryomarket/apps`
- generated desktop entries: `$XDG_DATA_HOME/applications`

## Safety boundaries

Ryomarket does not execute package commands through a shell. Native provider transactions use executable and argument arrays, and privileged steps are isolated behind `pkexec`.

Arbitrary `.deb` files and arbitrary tarballs are not treated as universal Linux packages. Portable archive support is recipe-driven, reversible and rejects path traversal and link entries during extraction.

See `docs/ARCHITECTURE.md` and `docs/CATALOG.md` for the internal model.

## License

Ryomarket follows the Ryoku project under the GNU General Public License version 3.
