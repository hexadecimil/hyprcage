# Development

## How it works

Hyprland keeps the human's screens. hyprcage adds a headless output far away
from them, pins a workspace to it and launches cage there through Hyprland's
own `exec` so that the window rules apply (silent workspace, no initial
focus). cage is a compositor with its own seat: hyprcage connects to it as a
Wayland client and drives it with the virtual-pointer, virtual-keyboard and
screencopy protocols, in pure Go. The human watches through a `wl-mirror`
window on a spare workspace.

Every screen runs in its own transient systemd slice, with a record in
`$XDG_RUNTIME_DIR/hyprcage` and a watcher that re-applies its geometry after
`hyprctl reload` or when a desktop bar lands on the output. Screens close
when the agent asks, when its session ends (hooks, stdin EOF) and, at the
latest, on a per-screen safety timer once the session is dead.

```
cmd/hyprcage          the binary
internal/hypr         Hyprland IPC, events, classic + Lua config drivers
internal/screen       create / launch / destroy / gc, capture, input
internal/wl           the Wayland client (pointer, keyboard, screencopy)
internal/registry     one record per screen
internal/session      who owns a screen, is that session alive
internal/sysd         systemd slices, scopes, timers
internal/config       config.toml
internal/setup        installing cage with the human's authorisation
internal/mcpserver    the MCP server
internal/cli          the command line
bin/, hooks/, skills/, .mcp.json, .claude-plugin/   the plugin
install.sh, .github/  the installer and the release workflow
tests/vm/             the VM test suite
```

## Configuration modes

Hyprland 0.56 ships two parsers. The driver is chosen at runtime
(`hyprctl repl "return 1"` answers `1` under Lua, and `HYPRCAGE_DRIVER` overrides it).

| | classic | Lua |
|---|---|---|
| output and workspace rules | `hyprctl keyword …` | `hyprctl eval 'hl.monitor{…}'` |
| workspace rule keys | `gapsin`, `bordersize`, `rounding` | `gaps_in`, `border_size`, `no_rounding` |
| launch with rules | `dispatch exec [workspace 11 silent; no_initial_focus] …` | `hl.exec_cmd(cmd, {…})` |
| dispatchers | `dispatch focusmonitor X` | `dispatch hl.dsp.focus({ monitor = "X" })` |

## Desktop bars

A shell that puts its bar on every output (Omarchy) reserves part of the
agent's output. hyprcage enlarges the headless output by that reserved area
so that the cage window, which is what the agent sees, keeps its exact size.
Such bars re-create their layer around every output change, so the watcher
only follows a reserved area once it has held still, and `create` waits for
the bar to land before launching cage.

## The human's state

Everything hyprcage does to Hyprland is aimed at one output, and the rule
that separates that output from the human's monitors is the name: the
configured prefix, `hc-` by default, forced onto every screen name by
`Create`. The registry answers for a screen recorded before that rule
existed. Get this wrong and `screen_destroy` restores a workspace that only
ever lived on the agent's output, which leaves one of the human's monitors on
an empty workspace.

Removing an output makes Hyprland migrate its workspaces and can warp the
cursor. `destroy` reads the active workspace of every monitor of the human
plus the cursor and the focus, removes the output, then puts back whatever
moved, and keeps watching for two seconds because a desktop's own scripts
react after Hyprland does. Every move and every restoration is appended to
`~/.local/state/hyprcage/log/restore.log`.

## Monitor events and desktop shells

Declaring an output goes through Hyprland's monitor rules, which are applied
to every monitor: on a real machine that can cost a modeset, a monitor that
goes black for an instant. `Reassert` and `Refit` therefore read the live
geometry and declare nothing when it already matches, which leaves one
declaration per screen.

The rest is the shell's. Omarchy runs `omarchy-hyprland-monitor-watch`,
which on every `monitoradded` and `monitorremoved` re-runs its clamshell
logic four times over eleven seconds and can call `hyprctl reload`. Its
`omarchy-hyprland-monitor-external-active` counts any output not named
`eDP-*`, `LVDS-*` or `DSI-*` as an external display, an agent screen
included, so closing the lid of a laptop with no real external display while
a screen exists switches the panel off. None of this is reachable from
hyprcage. What hyprcage owes the human is to add and remove one output per
screen and nothing more.

## Renderer

cage uses GLES by default. hyprcage probes one capture before handing the
screen over and falls back to pixman when it does not complete, remembering
the choice in `~/.local/state/hyprcage/renderer`. Virtualized GPUs (virgl)
fail that probe with every renderer. The probe waits 15 seconds because a
loaded machine can delay a capture that works, and a renderer that was
remembered and then failed is forgotten again, so one bad night never pins a
machine to software rendering. A probe that times out answers
`capture_failed`, never `cage_missing`, which is the code that sends an
agent to `setup`.

## Test suite

Everything is validated in a VM, never on a desktop: a vanilla Arch guest
from the cloud image, driven over SSH.

```
tests/vm/vm.sh fetch && tests/vm/vm.sh init && tests/vm/vm.sh start && tests/vm/vm.sh provision
tests/vm/vm.sh ssh HYPR_CONFIG=classic ./hypr-start.sh          # or lua
make build && tests/vm/vm.sh scp hyprcage tests/vm/guest/spikes.sh guest:/home/arch/
tests/vm/vm.sh ssh EXPECT_DRIVER=classic ./spikes.sh
```

`HYPRCAGE_VM_PROFILE=omarchy HYPRCAGE_VM_SSH_PORT=2223` keeps a second guest
with the `omarchy` package. `guest/install-test.sh` exercises the installer.
`make dist` builds the release assets and a tag `v*` publishes them.
