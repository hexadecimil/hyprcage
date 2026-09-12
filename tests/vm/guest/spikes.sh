#!/usr/bin/env bash
# Phase-0 spikes (cahier §10) and the automatable acceptance checks (§9),
# run INSIDE the VM as user arch with Hyprland up. Prints PASS/FAIL/INFO and
# leaves screenshots in $HOME/spikes/. One application per screen: that is the
# supported model (cage is a single-application kiosk), so each app runs in its
# own freshly created cage, exactly as an agent would.
set -uo pipefail
H=${H:-$HOME/hyprcage}
export XDG_RUNTIME_DIR=/run/user/$(id -u)
export HYPRLAND_INSTANCE_SIGNATURE=${HYPRLAND_INSTANCE_SIGNATURE:-$(ls "$XDG_RUNTIME_DIR/hypr" | head -1)}
OUT=$HOME/spikes; mkdir -p "$OUT"
FAILED=0
pass() { echo "PASS $*"; }
fail() { echo "FAIL $*"; FAILED=1; }
info() { echo "INFO $*"; }

snap() { echo "cursor=$(hyprctl -j cursorpos | jq -c .) monitors=$(hyprctl -j monitors | jq -c '[.[] | select(.name|startswith("hc-")|not) | {name, ws: .activeWorkspace.id, focused}]')"; }
create() { $H create --json "$@"; }
launch() { $H launch "$1" -- "${@:2}" >/dev/null; }
wait_title() { $H wait "$1" --title "$2" --timeout "${3:-10s}" >/dev/null 2>&1; }
evwait() { local f=$1 re=$2 n=${3:-40} i; for ((i=0;i<n;i++)); do local m; m=$(grep -aE "$re" "$f" 2>/dev/null | tail -1); [ -n "$m" ] && { echo "$m"; return 0; }; sleep 0.1; done; return 1; }
evcount() { grep -acE "$1" "$2" 2>/dev/null || echo 0; }

echo "== doctor"
$H doctor; drv=$($H doctor --json | jq -r '.[]|select(.name=="config driver")|.detail' | awk '{print $1}'); info "config driver: $drv"
# EXPECT_DRIVER=lua|classic asserts the mode the VM was started in (hypr-start.sh HYPR_CONFIG).
if [ -n "${EXPECT_DRIVER:-}" ]; then [ "$drv" = "$EXPECT_DRIVER" ] && pass "S8 config driver detected as $drv" || fail "S8 config driver is $drv, expected $EXPECT_DRIVER"; fi

# HUMAN_MONITORS=2 adds a second monitor for the human (a headless output
# outside hyprcage's prefix), placed to the right, so that the migration of
# workspaces at `output remove` has two possible targets, as on a laptop
# with an external display.
if [ "${HUMAN_MONITORS:-1}" = 2 ] && ! hyprctl -j monitors | jq -e '.[]|select(.name=="hm-2")' >/dev/null; then
  if [ "$drv" = lua ]; then hyprctl eval 'hl.monitor({ output = "hm-2", mode = "1920x1080@60", position = "1280x0", scale = 1 })' >/dev/null; else hyprctl keyword monitor 'hm-2,1920x1080@60,1280x0,1' >/dev/null; fi
  hyprctl output create headless hm-2 >/dev/null; sleep 1
  info "second human monitor: $(hyprctl -j monitors | jq -c '.[]|select(.name=="hm-2")|{width,height,x,y,ws:.activeWorkspace.id}')"
fi
before=$(snap); info "human state before: $before"
ws_before=$(hyprctl -j workspaces | jq -c '[.[]|select(.id<=10)|.id]|sort')

echo "== S1/S3/S6/N3: create a screen (with mirror)"
out=$(create); name=$(echo "$out" | jq -r .name); echo "$out" | jq -c '{name,width,height,ws_app,ws_mirror,cage_pid}'
# A bar on every output (Omarchy) reserves part of it, possibly a few seconds after creation: the
# output is then enlarged by that area by the watcher. Let it settle before measuring.
# Settled = the cage window is 1280x800 and the output equals 1280x800 plus its reserved area, for 2 s in a row
# (a bar re-creates its layer around every output change, so the geometry can flap for a moment).
settle_cage() { local i ok=0; for i in $(seq 1 80); do if [ "$(hyprctl -j clients | jq -c ".[]|select(.workspace.id==11)|.size")" = "[1280,800]" ] && [ "$(hyprctl -j monitors | jq -c ".[]|select(.name==\"$name\")|(.width == 1280 + .reserved[0] + .reserved[2] and .height == 800 + .reserved[1] + .reserved[3])")" = true ]; then ok=$((ok+1)); [ $ok -ge 8 ] && return 0; else ok=0; fi; sleep 0.25; done; return 1; }
settle_cage || info "cage window / output did not settle within 15 s"
mon=$(hyprctl -j monitors | jq -c ".[]|select(.name==\"$name\")|{width,height,scale,x,y,reserved}")
exp_w=$(echo "$mon" | jq '1280 + .reserved[0] + .reserved[2]'); exp_h=$(echo "$mon" | jq '800 + .reserved[1] + .reserved[3]')
[ "$(echo "$mon"|jq .width)" = "$exp_w" ] && [ "$(echo "$mon"|jq .height)" = "$exp_h" ] && [ "$(echo "$mon"|jq .scale)" = 1 ] && pass "S1 output ${exp_w}x${exp_h} scale 1 (reserved $(echo "$mon"|jq -c .reserved))" || fail "S1 geometry $mon"
win=$(hyprctl -j clients | jq -c ".[]|select(.workspace.id==11)|{title,size}")
[ "$(echo "$win"|jq -c .size)" = "[1280,800]" ] && pass "S6 cage window on workspace 11 via exec rules, exactly 1280x800 ($win)" || fail "S6 cage window on workspace 11: $win"
adj=$(hyprctl -j monitors | jq -r --arg n "$name" '(.[]|select(.name==$n)) as $h | [.[]|select(.name!=$n)|select((($h.x==.x+(.width/.scale|floor) or .x==$h.x+$h.width) and ($h.y<.y+(.height/.scale|floor) and .y<$h.y+$h.height)) or (($h.y==.y+(.height/.scale|floor) or .y==$h.y+$h.height) and ($h.x<.x+(.width/.scale|floor) and .x<$h.x+$h.width)))|.name]|length')
[ "$adj" = 0 ] && pass "S3 output shares no edge with a real monitor" || fail "S3 adjacent to $adj monitor(s)"
[ "$before" = "$(snap)" ] && pass "N3 human state unchanged by create" || fail "N3 create changed the human state: $(snap)"
mir=""; for _ in $(seq 1 10); do mir=$(hyprctl -j clients | jq -c ".[]|select(.workspace.id==6 and .title==\"Wayland Output Mirror for $name\")|{size}"); [ -n "$mir" ] && break; sleep 0.5; done
[ -n "$mir" ] && pass "mirror window on workspace 6 ($mir)" || fail "no mirror window on workspace 6"

echo "== S5/N11: hyprctl reload re-asserts the geometry"
hyprctl reload >/dev/null; sleep 2; settle_cage || info "did not settle after reload"
mon2=$(hyprctl -j monitors | jq -c ".[]|select(.name==\"$name\")|{width,height,scale,x,y,reserved}")
[ "$mon" = "$mon2" ] && pass "N11 geometry re-asserted after reload ($mon2)" || fail "N11 reload left the output at $mon2"
$H shot "$name" -o "$OUT/00-after-reload.png" >/dev/null && [ -s "$OUT/00-after-reload.png" ] && pass "capture still works after reload" || fail "capture after reload"

echo "== S10/A5: destroy restores the human state and leaves nothing"
b=$(snap); $H destroy "$name" && pass "destroy ok" || fail "destroy"; sleep 1
[ "$b" = "$(snap)" ] && pass "S10 human state identical after destroy" || fail "S10 destroy changed the human state"
[ -z "$(pgrep -x cage)$(pgrep -x wl-mirror)" ] && pass "A5 no cage/wl-mirror process left" || fail "A5 processes left"
[ -z "$(systemctl --user list-units --plain --no-legend 'hyprcage-*' | awk '{print $1}')" ] && pass "A5 no hyprcage-* unit left" || fail "A5 units left"
[ -z "$(hyprctl -j monitors | jq -r '.[]|select(.name|startswith("hc-"))|.name')" ] && pass "A5 no hc-* output left" || fail "A5 outputs left"
[ "$ws_before" = "$(hyprctl -j workspaces | jq -c '[.[]|select(.id<=10)|.id]|sort')" ] && pass "A5 workspaces <=10 unchanged" || fail "A5 workspaces changed"

echo "== S2/A7/A11/A13: input, one wev per screen"
out=$(create --no-mirror); name=$(echo "$out" | jq -r .name)
rm -f "$HOME/wev.log"; launch "$name" sh -c "exec stdbuf -oL wev > $HOME/wev.log 2>&1"
wait_title "$name" '^wev$' 15s && pass "A app_launch: wev listed by foreign-toplevel" || fail "wev not listed"
evwait "$HOME/wev.log" 'wl_seat|wl_pointer|wl_keyboard' 100 >/dev/null && pass "wev connected to the cage seat" || fail "wev produced no output"
$H shot "$name" -o "$OUT/01-wev.png" >/dev/null && [ -s "$OUT/01-wev.png" ] && pass "S2 screencopy capture written" || fail "S2 shot"
$H click "$name" 640 400
{ evwait "$HOME/wev.log" 'x, y: 640\.0+, 400\.0+' >/dev/null && [ "$(evcount 'button: 272 \(left\)' "$HOME/wev.log")" -ge 2 ]; } && pass "A7 left click at exactly 640,400" || fail "A7 click"
$H click "$name" 100 50 --button right
{ evwait "$HOME/wev.log" 'x, y: 100\.0+, 50\.0+' >/dev/null && evwait "$HOME/wev.log" 'button: 273 \(right\), state: 0' >/dev/null; } && pass "A7 right click at 100,50" || fail "A7 right click"
n0=$(evcount 'button: 272 \(left\), state: 1' "$HOME/wev.log"); $H click "$name" 300 300 --count 2; sleep 0.5
[ $(($(evcount 'button: 272 \(left\), state: 1' "$HOME/wev.log")-n0)) = 2 ] && pass "A double click: two presses" || fail "double click"
lines0=$(wc -l < "$HOME/wev.log"); $H type "$name" 'Grüezi, ça va ? «été» 😀'; sleep 0.5
typed=$(tail -n +"$((lines0+1))" "$HOME/wev.log" | grep -aoE "utf8: '[^']*'" | sed -E "s/^utf8: '(.*)'\$/\1/" | tr -d '\n')
[ "$typed" = 'Grüezi, ça va ? «été» 😀' ] && pass "A11 Unicode typing verbatim" || fail "A11 typed '$typed'"
lines0=$(wc -l < "$HOME/wev.log"); $H key "$name" ctrl+b; sleep 0.4; seg=$(tail -n +"$((lines0+1))" "$HOME/wev.log")
{ echo "$seg" | grep -qE "key: 37; state: 1" && echo "$seg" | grep -qE "key: 37; state: 0"; } && pass "A13 ctrl+b: Control held around the letter" || fail "A13 ctrl+b"
lines0=$(wc -l < "$HOME/wev.log"); $H click "$name" 50 50 --mod ctrl; sleep 0.4; seg=$(tail -n +"$((lines0+1))" "$HOME/wev.log")
ci=$(echo "$seg" | grep -nE "key: 37; state: 1" | head -1 | cut -d: -f1); bi=$(echo "$seg" | grep -nE "button: 272 \(left\), state: 1" | head -1 | cut -d: -f1); ri=$(echo "$seg" | grep -nE "key: 37; state: 0" | tail -1 | cut -d: -f1)
{ [ -n "$ci" ] && [ -n "$bi" ] && [ -n "$ri" ] && [ "$ci" -lt "$bi" ] && [ "$bi" -lt "$ri" ]; } && pass "A13 ctrl+click: Control held across the button" || fail "ctrl+click order ci=$ci bi=$bi ri=$ri"
lines0=$(wc -l < "$HOME/wev.log"); $H scroll "$name" 200 300 down --amount 2; sleep 0.4
[ "$(tail -n +"$((lines0+1))" "$HOME/wev.log" | grep -acE 'axis')" -ge 2 ] && pass "A scroll: axis events delivered" || fail "scroll"
lines0=$(wc -l < "$HOME/wev.log"); $H drag "$name" 100 100 500 400; sleep 0.4; seg=$(tail -n +"$((lines0+1))" "$HOME/wev.log")
{ echo "$seg" | grep -qE 'x, y: 500\.0+, 400\.0+' && [ "$(echo "$seg" | grep -acE 'motion:')" -ge 10 ]; } && pass "A drag: interpolated motion, release at 500,400" || fail "drag"
wid=$($H windows "$name" --json | jq -r '.[]|select(.AppID=="wev")|.ID'|head -1); $H close "$name" "$wid" >/dev/null 2>&1; sleep 1
[ "$($H windows "$name" --json | jq -r '[.[]|.AppID]|index("wev")')" = null ] && pass "A app_close closes wev gracefully" || fail "wev still listed after close"
$H destroy "$name" >/dev/null

echo "== A13: modifiers interpreted end to end (Ctrl+D = EOF), one foot per screen"
out=$(create --no-mirror); name=$(echo "$out" | jq -r .name); rm -f /home/arch/catout
launch "$name" foot sh -c "cat > /home/arch/catout; echo DONE >> /home/arch/catout"
wait_title "$name" 'foot' 12s; sleep 1.5
$H type "$name" "hello-ctrl-d"; $H key "$name" Return; sleep 0.3; $H key "$name" ctrl+d; sleep 0.6
{ grep -q hello-ctrl-d /home/arch/catout 2>/dev/null && grep -q DONE /home/arch/catout 2>/dev/null; } && pass "A13 Ctrl+D delivered EOF (modifier interpreted)" || fail "Ctrl+D: $(cat /home/arch/catout 2>/dev/null | tr '\n' '|')"
$H destroy "$name" >/dev/null

echo "== S4: continuous rendering with nobody watching, one foot per screen"
out=$(create --no-mirror); name=$(echo "$out" | jq -r .name)
launch "$name" foot sh -c 'while true; do date +%N; sleep 0.2; done'
wait_title "$name" 'foot' 12s; sleep 2
$H shot "$name" -o "$OUT/02-a.png" >/dev/null; sleep 1; $H shot "$name" -o "$OUT/03-b.png" >/dev/null
{ [ -s "$OUT/02-a.png" ] && [ -s "$OUT/03-b.png" ] && ! cmp -s "$OUT/02-a.png" "$OUT/03-b.png"; } && pass "S4 the ticking terminal keeps rendering unwatched" || fail "S4 rendering frozen"
$H wait "$name" --stable 300ms --timeout 3s >/dev/null 2>&1 && fail "wait --stable returned on a changing screen" || pass "wait --stable times out on a changing screen"
wid=$($H windows "$name" --json | jq -r '.[0].ID'); $H close "$name" "$wid" >/dev/null 2>&1; sleep 1
$H wait "$name" --stable 300ms --timeout 5s >/dev/null 2>&1 && pass "wait --stable returns on a static screen" || fail "wait --stable timed out on a static screen"
$H destroy "$name" >/dev/null

echo "== gc reaps the output of a crashed cage"
out=$(create --no-mirror); name=$(echo "$out" | jq -r .name); kill -9 "$(echo "$out" | jq -r .cage_pid)"; sleep 1
$H gc >/dev/null; [ -z "$(hyprctl -j monitors | jq -r '.[]|select(.name|startswith("hc-"))|.name')" ] && pass "gc removed the output of a crashed cage" || fail "gc left an output"

echo "== config.toml honoured (size, workspaces, screens per session)"
mkdir -p ~/.config/hyprcage; cat > ~/.config/hyprcage/config.toml <<'EOT'
[screen]
width = 1024
height = 640
max_per_session = 1
[workspaces]
agent = [20, 30]
mirror = [7, 8]
EOT
$H doctor --json | jq -e '.[]|select(.name=="config")|select(.status=="ok" and (.detail|test("^/")))' >/dev/null && pass "doctor reports the config file" || fail "doctor: $($H doctor --json | jq -c '.[]|select(.name=="config")')"
out=$(create); name=$(echo "$out" | jq -r .name)
[ "$(echo "$out" | jq -c '[.ws_app,.ws_mirror,.width,.height]')" = "[20,7,1024,640]" ] && pass "config: workspaces 20/7 and size 1024x640 from config.toml" || fail "config not honoured: $(echo "$out" | jq -c '[.ws_app,.ws_mirror,.width,.height]')"
if create --no-mirror >/dev/null 2>"$OUT/limit.err"; then fail "config: max_per_session = 1 not enforced"; else grep -q 'limit' "$OUT/limit.err" && pass "config: max_per_session = 1 enforced ($(head -c 60 "$OUT/limit.err"))" || fail "config: second create failed for another reason: $(cat "$OUT/limit.err")"; fi
$H destroy "$name" >/dev/null
create --size 4000x100 >/dev/null 2>"$OUT/size.err" && fail "config: size limit not enforced" || pass "config: size outside the limits refused"
printf '[screen]\nwidht = 1\n' > ~/.config/hyprcage/config.toml
create >/dev/null 2>"$OUT/typo.err" && fail "config: typo accepted" || { grep -q 'unknown key' "$OUT/typo.err" && pass "config: unknown key refused" || fail "config: typo error is $(cat "$OUT/typo.err")"; }
$H gc >/dev/null 2>&1 && pass "config: gc still runs on a broken config (defaults)" || fail "config: gc failed on a broken config"
rm -f ~/.config/hyprcage/config.toml

echo; [ "$FAILED" = 0 ] && echo "ALL PASS" || echo "SOME FAILURES"; exit $FAILED
