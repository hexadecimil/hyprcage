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

Omarchy's lid logic counts any output not named `eDP-*`, `LVDS-*` or `DSI-*`
as external, an agent screen included: closing the lid of a laptop without a
real external display while a screen exists switches the panel off.

## Renderer

cage uses GLES by default. hyprcage probes one capture before handing the
screen over and falls back to pixman when it does not complete, remembering
the choice in `~/.local/state/hyprcage/renderer`. Virtualized GPUs (virgl)
fail that probe with every renderer.

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
