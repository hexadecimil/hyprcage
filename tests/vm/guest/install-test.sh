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
sudo pacman -R --noconfirm cage wl-mirror >/dev/null 2>&1; rm -f ~/.local/bin/hyprcage
command -v cage >/dev/null && fail "precondition: cage still installed"

echo "== install.sh end to end (packages by sudo -n, binary from the release base)"
bash "$R/install.sh" > ~/install.log 2>&1 && pass "install.sh exits 0" || { fail "install.sh failed"; tail -5 ~/install.log; }
command -v cage >/dev/null && pass "cage installed" || fail "cage not installed"
command -v wl-mirror >/dev/null && pass "wl-mirror installed" || fail "wl-mirror not installed"
[ -x ~/.local/bin/hyprcage ] && pass "binary in ~/.local/bin ($(hyprcage version))" || fail "binary missing"
grep -q 'plugin marketplace add' ~/install.log && pass "plugin commands printed (no claude CLI here)" || fail "plugin step: $(grep -i plugin ~/install.log | head -2)"
grep -q '^hyprcage  *ok' ~/install.log && pass "doctor ran at the end" || fail "doctor did not run: $(tail -3 ~/install.log)"

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
command -v cage >/dev/null && pass "uninstall leaves cage, as documented" || fail "uninstall removed cage"
bash "$R/install.sh" --binary-only >/dev/null 2>&1 || true   # back in place for the other suites
echo; [ $FAILED = 0 ] && echo "ALL PASS" || echo "SOME FAILURES"; exit $FAILED
