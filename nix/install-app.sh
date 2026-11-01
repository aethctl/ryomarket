umask 077
state_base="${XDG_STATE_HOME:-$HOME/.local/state}/ryomarket/flake-apps"
data_base="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
root="$state_base/$RYOMARKET_APP_NAME"
desktop="$data_base/dev.ryoku.ryomarket.$RYOMARKET_APP_NAME.desktop"
owner="ryomarket:$RYOMARKET_APP_NAME"
bin_base="${XDG_BIN_HOME:-$HOME/.local/bin}"
mkdir -p "$state_base/.locks"
exec 9>"$state_base/.locks/$RYOMARKET_APP_NAME.lock"
flock 9
if [[ -L "$root" ]] || { [[ -e "$root" ]] && [[ "$(cat "$root/managed-by" 2>/dev/null || true)" != "$owner" ]]; }; then
  echo "Refusing to modify an unmanaged application directory: $root" >&2
  exit 1
fi
if [[ "${1:-}" == "--remove" ]]; then
  if [[ ! -e "$root" ]]; then exit 0; fi
  if [[ -L "$desktop" ]] && [[ "$(readlink "$desktop")" == "$root/current/app.desktop" ]]; then rm -- "$desktop"; fi
  if [[ -f "$root/current/commands" && -f "$root/current/bin-dir" ]]; then
    installed_bin=$(cat "$root/current/bin-dir")
    while IFS= read -r command_name; do
      target="$installed_bin/$command_name"
      if [[ -L "$target" && "$(readlink "$target")" == "$root/current/profile/bin/$command_name" ]]; then rm -- "$target"; fi
    done < "$root/current/commands"
  fi
  rm -rf -- "$root"
  echo "Removed $RYOMARKET_APP_NAME from its RyoMarket profile."
  exit 0
fi
if [[ $# -ne 0 ]]; then echo 'Usage: installer [--remove]' >&2; exit 2; fi
if [[ -e "$desktop" || -L "$desktop" ]] && { [[ ! -L "$desktop" ]] || [[ "$(readlink "$desktop")" != "$root/current/app.desktop" ]]; }; then
  echo "Refusing to replace an unmanaged desktop entry: $desktop" >&2
  exit 1
fi
mkdir -p "$root/generations" "$data_base"
printf '%s\n' "$owner" > "$root/managed-by"
generation=$(mktemp -d "$root/generations/install.XXXXXXXX")
published=0
cleanup() {
  if [[ "$published" -eq 0 ]]; then rm -rf -- "$generation"; fi
}
trap cleanup EXIT
nix --extra-experimental-features 'nix-command flakes' profile add \
  --profile "$generation/profile" \
  "$RYOMARKET_APP_FLAKE#$RYOMARKET_APP_NAME"
test -x "$generation/profile/bin/$RYOMARKET_APP_PROGRAM"
mkdir -p "$bin_base"
: > "$generation/commands"
printf '%s\n' "$bin_base" > "$generation/bin-dir"
for binary in "$generation/profile/bin/"*; do
  [[ -x "$binary" && ! -d "$binary" ]] || continue
  command_name=$(basename "$binary")
  [[ "$command_name" =~ ^[a-zA-Z0-9_+.-]+$ ]] || { echo 'Unsupported executable name' >&2; exit 1; }
  target="$bin_base/$command_name"
  if [[ -e "$target" || -L "$target" ]] && { [[ ! -L "$target" ]] || [[ "$(readlink "$target")" != "$root/current/profile/bin/$command_name" ]]; }; then
    echo "Refusing to replace an unmanaged command: $target" >&2
    exit 1
  fi
  printf '%s\n' "$command_name" >> "$generation/commands"
done
printf '%s\n' "$RYOMARKET_APP_FLAKE" > "$generation/source"
printf '[Desktop Entry]\nType=Application\nName=%s\nExec="%s"\nIcon=%s\nTerminal=false\nCategories=Utility;\n' \
  "$RYOMARKET_APP_TITLE" "$root/current/profile/bin/$RYOMARKET_APP_PROGRAM" "$RYOMARKET_APP_ICON" > "$generation/app.desktop"
ln -s "$generation" "$generation/current-link"
mv -Tf -- "$generation/current-link" "$root/current"
published=1
while IFS= read -r command_name; do
  ln -sfn "$root/current/profile/bin/$command_name" "$bin_base/$command_name"
done < "$generation/commands"
ln -sfn "$root/current/app.desktop" "$desktop"
if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database "$data_base" || true; fi
echo "Installed $RYOMARKET_APP_TITLE. Open it from your application launcher."
