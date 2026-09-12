#!/usr/bin/env bash
# Start Hyprland detached from an ssh session, with seatd as the seat backend
# (no VT, no logind session) and software rendering on virtio-gpu.
#   HYPR_CONFIG=classic   ~/.config/hypr/hyprland.conf (default)
#   HYPR_CONFIG=lua       ~/.config/hypr/hyprland.lua
#   HYPR_CONFIG=omarchy   Omarchy's ~/.config/hypr/hyprland.lua (omarchy-setup.sh)
set -euo pipefail
export XDG_RUNTIME_DIR=/run/user/$(id -u)
export LIBSEAT_BACKEND=seatd
export WLR_NO_HARDWARE_CURSORS=1
export XDG_SESSION_TYPE=wayland
mkdir -p "$XDG_RUNTIME_DIR"
if pgrep -x Hyprland >/dev/null; then echo "Hyprland already running"; exit 0; fi
# Instance directories of previous runs would be found first below.
for d in "$XDG_RUNTIME_DIR"/hypr/*/; do [ -S "$d.socket.sock" ] || rm -rf "$d"; done 2>/dev/null
case ${HYPR_CONFIG:-classic} in
  classic) cfg=$HOME/.config/hypr/hyprland.conf ;;
  lua|omarchy) cfg=$HOME/.config/hypr/hyprland.lua ;;
  *) echo "unknown HYPR_CONFIG=$HYPR_CONFIG" >&2; exit 2 ;;
esac
[ -f "$cfg" ] || { echo "missing $cfg" >&2; exit 2; }
nohup Hyprland -c "$cfg" > "$HOME/hyprland.log" 2>&1 &
for i in $(seq 1 60); do
  sig=$(ls "$XDG_RUNTIME_DIR/hypr" 2>/dev/null | head -1 || true)
  if [ -n "$sig" ] && [ -S "$XDG_RUNTIME_DIR/hypr/$sig/.socket.sock" ]; then
    echo "HYPRLAND_INSTANCE_SIGNATURE=$sig"
    exit 0
  fi
  sleep 1
done
echo "Hyprland did not come up; tail of the log:" >&2; tail -30 "$HOME/hyprland.log" >&2; exit 1
