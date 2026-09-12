#!/usr/bin/env bash
# Runs as root inside the guest: everything phase 0 needs, classic Hyprland
# configuration (this VM is the "Arch vanilla" environment of the spec).
set -euo pipefail
pacman -Syu --noconfirm --needed \
  hyprland cage wl-mirror grim wtype wev foot chromium xorg-xwayland socat \
  seatd mesa ttf-dejavu noto-fonts jq
systemctl enable --now seatd
usermod -aG seat,video,input arch
install -d -o arch -g arch /home/arch/.config /home/arch/.config/hypr
install -o arch -g arch -m 0644 /home/arch/hyprland.conf /home/arch/.config/hypr/hyprland.conf
install -o arch -g arch -m 0644 /home/arch/hyprland.lua /home/arch/.config/hypr/hyprland.lua
echo "provisioned: $(pacman -Q hyprland cage wl-mirror | tr '\n' ' ')"
