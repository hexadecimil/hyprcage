#!/usr/bin/env bash
# hyprcage installer: one command, everything in place.
#
#   curl -fsSL https://raw.githubusercontent.com/hexadecimil/hyprcage/main/install.sh | bash
#
# What it does, in order, skipping what is already there:
#   1. cage (the agent's compositor) and wl-mirror (the human's mirror window),
#      through pacman. sudo asks for your password once.
#   2. the hyprcage binary for this machine, from the GitHub release, checksum
#      verified against the SHA256SUMS published with it, into ~/.local/bin.
#   3. the Claude Code plugin (this repository is its own marketplace), when
#      the claude CLI is present; otherwise it prints the two commands.
#   4. hyprcage doctor.
#
# Options and environment:
#   --binary-only            step 2 only (what the plugin's launcher runs)
#   --uninstall              remove the binary, the plugin and hyprcage's state
#   HYPRCAGE_VERSION=vX.Y.Z  pin a release (default: the latest)
#   HYPRCAGE_FROM_SOURCE=1   build with go instead of downloading
#   HYPRCAGE_BIN_DIR=DIR     where the binary goes (default ~/.local/bin)
#   HYPRCAGE_RELEASE_BASE=URL  where the assets are fetched from (tests; default
#                            the GitHub release of the version)
set -euo pipefail

REPO=${HYPRCAGE_REPO:-hexadecimil/hyprcage}
BIN_DIR=${HYPRCAGE_BIN_DIR:-$HOME/.local/bin}
SRC_DIR=$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || echo "")
PACKAGES=(cage wl-mirror)

say() { printf '\033[1;36m==>\033[0m %s\n' "$*" >&2; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
die() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

arch() {
  case $(uname -m) in
    x86_64) echo amd64 ;;
    aarch64 | arm64) echo arm64 ;;
    *) die "unsupported architecture $(uname -m) (releases cover x86_64 and aarch64)" ;;
  esac
}

# --- 1. packages -------------------------------------------------------------

missing_packages() {
  local p; for p in "${PACKAGES[@]}"; do have "$p" || echo "$p"; done
}

install_packages() {
  local missing; mapfile -t missing < <(missing_packages)
  if [ ${#missing[@]} -eq 0 ]; then say "cage and wl-mirror already installed"; return; fi
  have pacman || die "cage is missing and this is not an Arch-based system: install ${missing[*]} with your package manager, then rerun"
  say "installing ${missing[*]} (sudo will ask for your password)"
  if sudo -n true 2>/dev/null || [ -t 0 ]; then
    sudo pacman -S --needed --noconfirm "${missing[@]}" || die "pacman failed"
  elif have pkexec; then
    pkexec pacman -S --needed --noconfirm "${missing[@]}" || die "pacman failed"
  else
    die "no terminal for sudo and no pkexec: run  sudo pacman -S ${missing[*]}  then rerun"
  fi
}

# --- 2. binary -------------------------------------------------------------------

latest_version() {
  # The release page of "latest" redirects to the tagged one.
  curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed 's|.*/tag/||'
}

build_from_source() {
  have go || return 1
  local src=$SRC_DIR
  if [ ! -f "$src/go.mod" ]; then
    src=$(mktemp -d); say "cloning $REPO"; git clone -q --depth 1 "https://github.com/$REPO" "$src" || return 1
  fi
  say "building from source in $src"
  (cd "$src" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/hexadecimil/hyprcage/internal/version.Version=$(git -C "$src" describe --tags --always 2>/dev/null || echo source)" -o "$BIN_DIR/hyprcage" ./cmd/hyprcage)
}

install_binary() {
  mkdir -p "$BIN_DIR"
  if [ "${HYPRCAGE_FROM_SOURCE:-0}" = 1 ]; then build_from_source || die "build failed"; return; fi
  local version=${HYPRCAGE_VERSION:-} a tmp
  a=$(arch)
  if [ -z "$version" ]; then version=$(latest_version) || true; fi
  [ -n "$version" ] || die "cannot find the latest release of $REPO (offline?); HYPRCAGE_FROM_SOURCE=1 builds it with go"
  if [ -x "$BIN_DIR/hyprcage" ] && [ "$("$BIN_DIR/hyprcage" version 2>/dev/null)" = "$version" ]; then
    say "hyprcage $version already in $BIN_DIR"; return
  fi
  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' RETURN
  local base=${HYPRCAGE_RELEASE_BASE:-"https://github.com/$REPO/releases/download/$version"}
  say "downloading hyprcage $version for linux/$a"
  if ! curl -fsSL -o "$tmp/hyprcage-linux-$a" "$base/hyprcage-linux-$a" || ! curl -fsSL -o "$tmp/SHA256SUMS" "$base/SHA256SUMS"; then
    warn "download failed"; build_from_source || die "no release for $version and no go toolchain to build from source"; return
  fi
  (cd "$tmp" && sha256sum -c --ignore-missing --quiet SHA256SUMS) || die "checksum mismatch for hyprcage-linux-$a: not installing"
  install -m 0755 "$tmp/hyprcage-linux-$a" "$BIN_DIR/hyprcage"
  say "installed $BIN_DIR/hyprcage ($("$BIN_DIR/hyprcage" version))"
}

check_path() {
  case ":$PATH:" in *":$BIN_DIR:"*) return ;; esac
  warn "$BIN_DIR is not on your PATH; add it to your shell profile and to your graphical session (Hyprland's env), or the plugin will not find hyprcage"
}

# --- 3. plugin -------------------------------------------------------------------

register_plugin() {
  if ! have claude; then
    say "claude CLI not found; in Claude Code run:  /plugin marketplace add $REPO  then  /plugin install hyprcage@hyprcage"
    return
  fi
  if claude plugin marketplace list 2>/dev/null | grep -q '^hyprcage\b\|hyprcage'; then
    say "marketplace hyprcage already known"
  else
    say "adding the plugin marketplace"; claude plugin marketplace add "$REPO" >/dev/null || warn "could not add the marketplace; run: claude plugin marketplace add $REPO"
  fi
  say "installing the plugin"; claude plugin install hyprcage@hyprcage >/dev/null 2>&1 && say "plugin installed (takes effect in a new session)" || warn "could not install the plugin; run: claude plugin install hyprcage@hyprcage"
}

# --- uninstall ---------------------------------------------------------------------

uninstall() {
  say "removing the plugin, the binary and hyprcage's state (cage and wl-mirror are left)"
  if have claude; then
    claude plugin uninstall hyprcage >/dev/null 2>&1 || true
    claude plugin marketplace remove hyprcage >/dev/null 2>&1 || true
  fi
  if [ -x "$BIN_DIR/hyprcage" ]; then "$BIN_DIR/hyprcage" gc --all >/dev/null 2>&1 || true; fi
  rm -f "$BIN_DIR/hyprcage"
  rm -rf "${XDG_STATE_HOME:-$HOME/.local/state}/hyprcage" "${XDG_CONFIG_HOME:-$HOME/.config}/hyprcage"
  say "done; to remove cage too:  sudo pacman -Rns cage wl-mirror"
}

# --- main ------------------------------------------------------------------------------

case ${1:-} in
  --binary-only) install_binary ;;
  --uninstall) uninstall ;;
  "")
    [ "$(uname -s)" = Linux ] || die "hyprcage runs on Linux with Hyprland"
    have Hyprland || warn "Hyprland not found in PATH; hyprcage needs a running Hyprland to do anything"
    install_packages
    install_binary
    check_path
    register_plugin
    say "checking the installation"
    "$BIN_DIR/hyprcage" doctor || true
    ;;
  *) die "usage: install.sh [--binary-only | --uninstall]" ;;
esac
