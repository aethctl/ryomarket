# Ryomarket architecture

Ryomarket separates application identity from installation providers.

```text
Go UI
  ↓
Unified catalog
  ↓
Provider resolver
  ↓
DNF · pacman · XBPS · Nix · Flatpak · AppImage · curated upstream
```

A catalog application has one stable Ryomarket ID. Each supported distro maps that identity to one or more provider variants. Provider package names never become user-facing duplicate applications.

The UI never constructs package-manager shell commands. It asks the service layer for a transaction plan. Providers return executable and argument arrays. The runner performs privileged native package operations through `pkexec` only when the provider requires it.

## Distro safety

The host distro is detected from `/etc/os-release`, but Discover uses one unified catalogue rather than separate distro views. Applications carry provider-scope markers for universal, Fedora, Arch/CachyOS, Void and Nix availability. Transactions only expose providers that are valid for the detected host.

Flatpak can bootstrap itself through DNF, pacman or XBPS when the host is Fedora, Arch/CachyOS or Void. The native prerequisite is part of the visible transaction plan. NixOS does not receive an imperative Flatpak bootstrap because that changes declarative system configuration.

## Portable software

AppImages and curated upstream archives are managed by `ryomarket-portable`. It owns their install locations, desktop entries and manifests. Archive recipes must define a known entrypoint, and extraction rejects absolute paths, traversal paths and links.

## NixOS

`nix profile` is treated as a user-scoped provider. Declarative NixOS installs use the separate `ryomarket-nix` helper and are exposed only when `/etc/nixos/configuration.nix` already imports `/etc/nixos/ryomarket.nix`. Ryomarket does not rewrite arbitrary flakes. Managed declarative edits are restored automatically if the NixOS rebuild fails.
