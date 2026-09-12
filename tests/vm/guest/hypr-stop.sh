#!/usr/bin/env bash
# Stop Hyprland and wait until it is gone, so that hypr-start.sh can start
# another one right away (with another configuration or scale).
pkill -x Hyprland || exit 0
for i in $(seq 1 20); do pgrep -x Hyprland >/dev/null || exit 0; sleep 0.5; done
pkill -9 -x Hyprland; sleep 1; exit 0
