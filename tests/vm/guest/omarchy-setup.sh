#!/usr/bin/env bash
# Runs as root inside the guest, after provision.sh: turns the vanilla VM into
# an Omarchy machine the way an Omarchy install is made of packages (same
# pacman repository and keyring), minus what only the ISO does: disk layout,
# limine as the actual bootloader, SDDM autologin. Hyprland is still started
# from ssh by hypr-start.sh (HYPR_CONFIG=omarchy) with Omarchy's Lua config.
set -euo pipefail
if ! grep -q '^\[omarchy\]' /etc/pacman.conf; then
  printf '\n[omarchy]\nSigLevel = Optional TrustAll\nServer = https://pkgs.omarchy.org/stable/$arch\n' >> /etc/pacman.conf
fi
pacman -Sy --noconfirm --needed omarchy-keyring
sed -i '/^\[omarchy\]/,/^Server/{/^SigLevel/d}' /etc/pacman.conf # back to the signed default
pacman -Sy --noconfirm --needed omarchy socat
systemctl disable --now sddm 2>/dev/null || true # the ISO's login manager; ssh + seatd here
# Per-user Hyprland config, as omarchy-refresh-hyprland does for a fresh account,
# and the first-login wizard marked done so it does not pop up during the tests.
u=arch; h=/home/$u
sudo -u $u mkdir -p "$h/.config/hypr" "$h/.local/state/omarchy/toggles/hypr"
sudo -u $u cp -f /usr/share/omarchy/config/hypr/* "$h/.config/hypr/"
sudo -u $u cp -f /usr/share/omarchy/default/hypr/toggles/flags.lua "$h/.local/state/omarchy/toggles/hypr/"
sudo -u $u env OMARCHY_PATH=/usr/share/omarchy PATH="/usr/share/omarchy/bin:$PATH" omarchy-done mark first-run-user || true
echo "omarchy-setup: $(pacman -Q omarchy hyprland | tr '\n' ' ')"
