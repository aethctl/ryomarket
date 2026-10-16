#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
prefix="${PREFIX:-$HOME/.local}"

cd "$repo_dir"
make build

install -Dm755 build/ryomarket "$prefix/bin/ryomarket"
install -Dm755 build/ryomarket-portable "$prefix/bin/ryomarket-portable"
install -Dm644 packaging/dev.ryoku.Ryomarket.desktop "$prefix/share/applications/dev.ryoku.Ryomarket.desktop"
install -Dm644 packaging/dev.ryoku.Ryomarket.metainfo.xml "$prefix/share/metainfo/dev.ryoku.Ryomarket.metainfo.xml"
install -Dm644 assets/ryomarket.svg "$prefix/share/icons/hicolor/scalable/apps/dev.ryoku.Ryomarket.svg"

printf 'Installed Ryomarket to %s\n' "$prefix"
