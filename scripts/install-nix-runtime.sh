#!/usr/bin/env bash
set -euo pipefail

source /etc/os-release
id="${ID:-}"
like=" ${ID_LIKE:-} "

if [[ "$id" == "nixos" ]]; then
  exit 0
fi

if [[ "$id" == "fedora" || "$like" == *" fedora "* ]]; then
  sudo dnf install -y nix nix-daemon
  sudo systemctl enable --now nix-daemon.service
elif [[ "$id" == "arch" || "$id" == "cachyos" || "$like" == *" arch "* ]]; then
  sudo pacman -S --needed --noconfirm nix
  sudo systemctl enable --now nix-daemon.service
elif [[ "$id" == "void" || "$like" == *" void "* ]]; then
  sudo xbps-install -Sy nix
  sudo ln -sfn /etc/sv/nix-daemon /var/service/nix-daemon
else
  printf 'Unsupported distro for Ryomarket Nix runtime bootstrap: %s\n' "${PRETTY_NAME:-$id}" >&2
  exit 1
fi

nix --extra-experimental-features 'nix-command flakes' --version
