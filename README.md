# hyprcage

A caged desktop for AI agents on Hyprland.

When an agent needs a graphical application for itself, to test a UI, take
screenshots or drive an app, it should not do it on your desktop. hyprcage
gives it a screen of its own: a headless Hyprland output running a nested
[cage](https://github.com/cage-kiosk/cage) compositor with its own seat. The
agent's clicks and keystrokes never touch your mouse, keyboard, focus or
workspaces. You can watch it work on a spare workspace, or not.

## Install

**Claude Code**

```
/plugin marketplace add hexadecimil/hyprcage
/plugin install hyprcage@hyprcage
```

The plugin fetches its binary on first start. If cage is missing, the agent
asks for it through a password dialog the first time it needs a screen.

**Terminal**

```
curl -fsSL https://raw.githubusercontent.com/hexadecimil/hyprcage/main/install.sh | bash
```

Installs cage, the latest release and the plugin. `install.sh --uninstall`
removes them. To pin everything to one version, script included:

```
curl -fsSL https://raw.githubusercontent.com/hexadecimil/hyprcage/v0.1.0/install.sh | HYPRCAGE_VERSION=v0.1.0 bash
```

**From source** (Go ≥ 1.24)

```
git clone https://github.com/hexadecimil/hyprcage && cd hyprcage
make build && install -Dm755 hyprcage ~/.local/bin/hyprcage
hyprcage setup && hyprcage doctor
```

## Usage

The agent gets `screen_create`, `app_launch`, `screenshot`, `click`, `type`,
`key`, `scroll`, `drag`, `wait`, `windows`, `screen_destroy` and a few more.
Its skill tells it to use a screen for anything it launches for itself, one
application per screen, and to close it when done.

Each screen opens a mirror window on one of your spare workspaces, 6 to 9
by default. Switch to it with your usual workspace binding to watch the
agent live. Nothing you do there reaches the agent. Every tool has a
command-line twin: `hyprcage create`, `launch`, `shot`, `click`, `type`,
`destroy`, `list`, `doctor`.

## Configuration

A screen is a virtual monitor of `width` × `height` pixels. Screenshots have
exactly that size and clicks use those coordinates. 1280x800 is what vision
models read without downscaling. Above 2000 px a side, screenshots are scaled
and clicks lose precision, hence the ceiling. Each screen is a compositor of
its own, on its own output, with one Hyprland workspace, so yours are never
touched. Its mirror is an ordinary window on one of your workspaces. The
defaults work as they are. `~/.config/hyprcage/config.toml` changes them,
every key optional.

```toml
[screen]
width = 1280          # width of a new screen, in pixels
height = 800          # height of a new screen, in pixels
max_width = 3840      # widest screen an agent may ask for
max_height = 2160     # tallest screen an agent may ask for
max_per_session = 4   # screens one session may keep open at once (0 = no limit)
mirror = true         # open a mirror window where you can watch a screen
notify = true         # desktop notification when a screen opens or closes

[workspaces]
agent = [11, 99]      # workspaces the agent's screens take, one each
mirror = [6, 9]       # workspaces the mirror windows open on, first free one

[lifecycle]
safety_timer = "15m"  # delay after which a screen of a dead session is closed

[cage]
renderer = "auto"     # cage renderer: auto, gles (GPU) or pixman (software)
```

## Compatibility

Arch Linux and derivatives, Hyprland ≥ 0.50 in either configuration mode
(`hyprland.conf` or `hyprland.lua`), any launcher, any desktop shell.
Verified on Hyprland 0.56 with vanilla Arch and with Omarchy. Not yet
tested: HiDPI outputs, aarch64 at runtime, GLES capture on bare metal.

Browsers and Electron apps are single-instance: launch them in a cage with
their own profile, or with yours closed.

## Security

hyprcage isolates the agent from your graphical session, not from your
system: applications in a cage run as you. See [SECURITY.md](SECURITY.md).

## Development

`make build`, `make test`. Architecture and the VM test suite:
[docs/development.md](docs/development.md).

## License

MIT
