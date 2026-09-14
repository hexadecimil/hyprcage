#!/usr/bin/env bash
# Installer checks, run INSIDE the VM as user arch: install.sh end to end
# against a release base (the seed server of vm.sh serves $VMDIR/seed/rel),
# the refusal of a tampered checksum, the plugin launcher's first-run
# bootstrap, `hyprcage setup`, and uninstall. Expects the repository's
# install.sh, bin/ and .claude-plugin/ under ~/repo (tar over vm.sh ssh).
#   HYPRCAGE_RELEASE_BASE=http://10.0.2.2:8123/rel HYPRCAGE_VERSION=test ./install-test.sh
set -uo pipefail
FAILED=0; pass() { echo "PASS $*"; }; fail() { echo "FAIL $*"; FAILED=1; }
R=$HOME/repo
export PATH=$HOME/.local/bin:$PATH XDG_RUNTIME_DIR=/run/user/$(id -u)
# Without a local base, the assets come from the GitHub release of HYPRCAGE_VERSION.
: "${HYPRCAGE_VERSION:?set HYPRCAGE_VERSION (vX.Y.Z, or test with HYPRCAGE_RELEASE_BASE)}"
export HYPRCAGE_RELEASE_BASE=${HYPRCAGE_RELEASE_BASE:-https://github.com/hexadecimil/hyprcage/releases/download/$HYPRCAGE_VERSION}
pgrep -x Hyprland >/dev/null || { HYPR_CONFIG=classic ./hypr-start.sh >/dev/null 2>&1; sleep 3; }
export HYPRLAND_INSTANCE_SIGNATURE=$(ls "$XDG_RUNTIME_DIR/hypr" | head -1)
sudo pacman -R --noconfirm cage >/dev/null 2>&1; rm -f ~/.local/bin/hyprcage
command -v cage >/dev/null && fail "precondition: cage still installed"
# Fake agents: codex and gemini as scripts that log their arguments, config
# directories for cursor, windsurf and opencode.
mkdir -p ~/.local/bin ~/.cursor ~/.codeium/windsurf ~/.config/opencode ~/.codex ~/.claude; rm -f ~/fake-agents.log
printf '#!/bin/sh\necho "codex $*" >> "$HOME/fake-agents.log"\ncase "$*" in "mcp add"*) printf "[mcp_servers.hyprcage]\\n" >> "$HOME/.codex/config.toml";; "mcp remove"*) : > "$HOME/.codex/config.toml";; esac\n' > ~/.local/bin/codex
printf '#!/bin/sh\necho "gemini $*" >> "$HOME/fake-agents.log"\n' > ~/.local/bin/gemini
chmod +x ~/.local/bin/codex ~/.local/bin/gemini; : > ~/.codex/config.toml
echo '{"mcpServers":{"other":{"command":"x"}}}' > ~/.cursor/mcp.json

echo "== install.sh end to end (packages by sudo -n, binary from the release base)"
bash "$R/install.sh" > ~/install.log 2>&1 && pass "install.sh exits 0" || { fail "install.sh failed"; tail -5 ~/install.log; }
command -v cage >/dev/null && pass "cage installed" || fail "cage not installed"

[ -x ~/.local/bin/hyprcage ] && pass "binary in ~/.local/bin ($(hyprcage version))" || fail "binary missing"
grep -q 'plugin marketplace add' ~/install.log && pass "Claude Code: plugin commands printed (~/.claude present, no claude CLI)" || fail "plugin step: $(grep -i plugin ~/install.log | head -2)"
grep -q '^hyprcage  *ok' ~/install.log && pass "doctor ran at the end" || fail "doctor did not run: $(tail -3 ~/install.log)"
[ -s ~/.config/hyprcage/config.toml ] && grep -q '^group = "session"' ~/.config/hyprcage/config.toml && pass "config written with the defaults" || fail "config: $(head -3 ~/.config/hyprcage/config.toml 2>&1)"
grep -q 'wrote .*config.toml' ~/install.log && pass "install.sh said where the config is" || fail "install.sh silent about the config"
sed -i 's/^width = 1280/width = 1024/' ~/.config/hyprcage/config.toml

echo "== agents registered"
bin=$HOME/.local/bin/hyprcage
grep -q "codex mcp add hyprcage -- $bin mcp" ~/fake-agents.log && pass "codex: mcp add called" || fail "codex: $(grep codex ~/fake-agents.log)"
grep -q "gemini mcp add -s user hyprcage $bin mcp" ~/fake-agents.log && pass "gemini: mcp add called" || fail "gemini: $(grep gemini ~/fake-agents.log)"
[ "$(jq -r '.mcpServers.hyprcage.command' ~/.cursor/mcp.json)" = "$bin" ] && [ "$(jq -r '.mcpServers.other.command' ~/.cursor/mcp.json)" = x ] && pass "cursor: entry added, others kept" || fail "cursor: $(cat ~/.cursor/mcp.json)"
[ "$(jq -r '.mcpServers.hyprcage.args[0]' ~/.codeium/windsurf/mcp_config.json)" = mcp ] && pass "windsurf: entry added" || fail "windsurf: $(cat ~/.codeium/windsurf/mcp_config.json 2>&1)"
[ "$(jq -r '.mcp.hyprcage.type' ~/.config/opencode/opencode.json)" = local ] && [ "$(jq -r '.mcp.hyprcage.command[1]' ~/.config/opencode/opencode.json)" = mcp ] && pass "opencode: entry added" || fail "opencode: $(cat ~/.config/opencode/opencode.json 2>&1)"
for d in .codex/skills .cursor/skills .codeium/windsurf/skills .config/opencode/skills; do [ -s ~/$d/hyprcage/SKILL.md ] || fail "skill missing in ~/$d"; done; [ -s ~/.codex/skills/hyprcage/SKILL.md ] && pass "skill copied to the agents' skill directories"

echo "== a tampered checksum is refused"
rm -f ~/.local/bin/hyprcage; rm -rf ~/bad; mkdir -p ~/bad
curl -fsSL -o ~/bad/hyprcage-linux-amd64 "$HYPRCAGE_RELEASE_BASE/hyprcage-linux-amd64"
echo "0000000000000000000000000000000000000000000000000000000000000000  hyprcage-linux-amd64" > ~/bad/SHA256SUMS
(cd ~/bad && python3 -m http.server 8999 --bind 127.0.0.1 >/dev/null 2>&1 &); sleep 1
HYPRCAGE_RELEASE_BASE=http://127.0.0.1:8999 bash "$R/install.sh" --binary-only > ~/bad.log 2>&1 && fail "tampered binary accepted" || { grep -q 'checksum mismatch' ~/bad.log && [ ! -e ~/.local/bin/hyprcage ] && pass "checksum mismatch refused, nothing installed" || fail "wrong failure: $(tail -2 ~/bad.log)"; }
pkill -f 'http.server 8999' >/dev/null 2>&1

echo "== plugin launcher: first run installs the binary, then serves MCP"
rm -f ~/.local/bin/hyprcage
bash "$R/bin/hyprcage-mcp" < /dev/null > ~/launcher.out 2> ~/launcher.err; rc=$?
[ -x ~/.local/bin/hyprcage ] && pass "launcher installed the binary on first run" || fail "launcher did not install: $(tail -2 ~/launcher.err)"
[ $rc = 0 ] && pass "launcher served MCP and exited cleanly on EOF" || fail "launcher rc=$rc: $(tail -2 ~/launcher.err)"
grep -q '^width = 1024' ~/.config/hyprcage/config.toml && pass "an edited config survives a reinstall" || fail "config overwritten: $(grep '^width' ~/.config/hyprcage/config.toml)"
echo "== plugin launcher: an outdated release is updated, a dev build is kept"
cp ~/.local/bin/hyprcage ~/hyprcage.real
printf '#!/bin/sh\n[ "$1" = version ] && echo v0.0.1\n' > ~/.local/bin/hyprcage; chmod +x ~/.local/bin/hyprcage
bash "$R/bin/hyprcage-mcp" < /dev/null > /dev/null 2> ~/launcher2.err
{ grep -q 'updating the binary' ~/launcher2.err && [ "$(~/.local/bin/hyprcage version)" != v0.0.1 ]; } && pass "launcher replaced an outdated binary" || fail "launcher kept v0.0.1: $(tail -2 ~/launcher2.err)"
printf '#!/bin/sh\n[ "$1" = version ] && echo dev\n' > ~/.local/bin/hyprcage; chmod +x ~/.local/bin/hyprcage
bash "$R/bin/hyprcage-mcp" < /dev/null > /dev/null 2> ~/launcher3.err
[ "$(~/.local/bin/hyprcage version)" = dev ] && pass "launcher leaves a dev build alone" || fail "launcher replaced a dev build: $(tail -2 ~/launcher3.err)"
mv ~/hyprcage.real ~/.local/bin/hyprcage
bash "$R/bin/hyprcage-hook" session-start < /dev/null >/dev/null 2>&1 && pass "hook shim runs the hook" || fail "hook shim failed"
mv ~/.local/bin/hyprcage ~/hyprcage.bak; bash "$R/bin/hyprcage-hook" session-start < /dev/null >/dev/null 2>&1 && pass "hook shim is silent without the binary" || fail "hook shim failed without the binary"; mv ~/hyprcage.bak ~/.local/bin/hyprcage

echo "== hyprcage setup"
sudo pacman -R --noconfirm cage >/dev/null 2>&1
out=$(hyprcage setup --json 2>&1)
{ echo "$out" | jq -e '.installed | index("cage")' >/dev/null && command -v cage >/dev/null; } && pass "setup installed cage via $(echo "$out" | jq -r .method)" || fail "setup: $out"
hyprcage setup | grep -q 'nothing to install' && pass "setup is idempotent" || fail "setup second run"
hyprcage doctor --json | jq -e '.[]|select(.name=="setup")' >/dev/null && fail "doctor still shows a setup row with nothing missing" || pass "doctor has no setup row when nothing is missing"

echo "== uninstall"
bash "$R/install.sh" --uninstall >/dev/null 2>&1
[ ! -e ~/.local/bin/hyprcage ] && [ ! -e ~/.local/state/hyprcage ] && pass "uninstall removed the binary and the state" || fail "uninstall left files"
grep -q "codex mcp remove hyprcage" ~/fake-agents.log && grep -q "gemini mcp remove -s user hyprcage" ~/fake-agents.log && pass "uninstall: codex and gemini unregistered" || fail "uninstall: $(grep remove ~/fake-agents.log)"
[ "$(jq -r '.mcpServers.hyprcage // "gone"' ~/.cursor/mcp.json)" = gone ] && [ "$(jq -r '.mcpServers.other.command' ~/.cursor/mcp.json)" = x ] && [ "$(jq -r '.mcp.hyprcage // "gone"' ~/.config/opencode/opencode.json)" = gone ] && pass "uninstall: JSON entries removed, others kept" || fail "uninstall: cursor=$(cat ~/.cursor/mcp.json) opencode=$(cat ~/.config/opencode/opencode.json)"
[ ! -e ~/.cursor/skills/hyprcage ] && [ ! -e ~/.codex/skills/hyprcage ] && pass "uninstall: skills removed" || fail "uninstall: skills left"
echo "== --agents limits the registration"
n=$(grep -c 'mcp add' ~/fake-agents.log); mkdir -p ~/.cursor ~/.config/opencode
bash "$R/install.sh" --agents cursor >/dev/null 2>&1
[ "$(jq -r '.mcpServers.hyprcage.command' ~/.cursor/mcp.json)" = "$bin" ] && pass "--agents cursor: cursor registered" || fail "--agents cursor: $(cat ~/.cursor/mcp.json)"
[ "$(jq -r '.mcp.hyprcage // "none"' ~/.config/opencode/opencode.json 2>/dev/null || echo none)" = none ] && [ "$(grep -c 'mcp add' ~/fake-agents.log)" = "$n" ] && pass "--agents cursor: nobody else touched" || fail "--agents cursor: others touched"
bash "$R/install.sh" --uninstall >/dev/null 2>&1
rm -f ~/.local/bin/codex ~/.local/bin/gemini; rm -rf ~/.cursor ~/.codeium ~/.config/opencode ~/.codex ~/.claude
command -v cage >/dev/null && pass "uninstall leaves cage, as documented" || fail "uninstall removed cage"
bash "$R/install.sh" --binary-only >/dev/null 2>&1 || true   # back in place for the other suites
echo; [ $FAILED = 0 ] && echo "ALL PASS" || echo "SOME FAILURES"; exit $FAILED
