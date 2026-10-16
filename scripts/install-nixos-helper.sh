#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

if [[ ! -x build/ryomarket-nix ]]; then
  printf 'build/ryomarket-nix is missing. Run make build first.\n' >&2
  exit 1
fi

sudo install -Dm755 build/ryomarket-nix /usr/local/bin/ryomarket-nix
sudo /usr/local/bin/ryomarket-nix setup

printf 'Ryomarket declarative installs are ready. Refresh status or relaunch Ryomarket.\n'
