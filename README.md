# hyprcage

A caged desktop for AI agents on Hyprland.

When an agent needs a graphical application for itself, to test a UI, take
screenshots or drive an app, it should not do it on your desktop. hyprcage
gives it a screen of its own: a headless Hyprland output running a nested
[cage](https://github.com/cage-kiosk/cage) compositor with its own seat. The
agent's clicks and keystrokes never touch your mouse, keyboard, focus or
workspaces. You can watch it work on a spare workspace, or not. It works on
any Hyprland desktop, Omarchy included, with Claude Code, Codex, Cursor,
Gemini CLI, Windsurf, OpenCode or any other MCP client.

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

Installs cage, the latest release, and registers hyprcage with every agent
found on the machine. Options come after `bash -s --`:

```
curl -fsSL https://raw.githubusercontent.com/hexadecimil/hyprcage/main/install.sh | bash -s -- --agents codex,cursor
```

| Option | Effect |
|---|---|
| `--agents claude,codex,gemini,cursor,windsurf,opencode` | register only these agents (default: every agent found) |
| `--agents none` | register no agent |
| `--uninstall` | remove the binary, the registrations and hyprcage's state |
| `HYPRCAGE_VERSION=v0.1.0` | install that release instead of the latest |
| `HYPRCAGE_FROM_SOURCE=1` | build with Go instead of downloading |

To pin the script itself to a version as well, fetch it from the tag:

```
curl -fsSL https://raw.githubusercontent.com/hexadecimil/hyprcage/v0.1.0/install.sh | HYPRCAGE_VERSION=v0.1.0 bash
```

**From source** (Go ≥ 1.24)

```
git clone https://github.com/hexadecimil/hyprcage && cd hyprcage
make build && install -Dm755 hyprcage ~/.local/bin/hyprcage
hyprcage setup && hyprcage doctor
```

## Agents

| Agent | Tools | Skill | Screens of a finished session |
|---|---|---|---|
| Claude Code | plugin | plugin | closed at once |
| Codex | `codex mcp add` | `~/.codex/skills` | closed by the safety timer |
| Cursor | `~/.cursor/mcp.json` | `~/.cursor/skills` | closed by the safety timer |
| Gemini CLI | `gemini mcp add` | | closed by the safety timer |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | `~/.codeium/windsurf/skills` | closed by the safety timer |
| OpenCode | `~/.config/opencode/opencode.json` | `~/.config/opencode/skills` | closed by the safety timer |
| Any MCP client | `hyprcage mcp` on stdio | `skills/hyprcage/SKILL.md` | closed by the safety timer |

The installer registers hyprcage with every agent it finds. To choose,
`install.sh --agents codex,cursor` limits it to those, and `--agents none`
skips the step. Only the Claude Code plugin carries session hooks, which
close an agent's screens the moment its session ends. Elsewhere, the safety
timer closes them within 15 minutes.

## Usage

The agent gets `screen_create`, `app_launch`, `screenshot`, `click`, `type`,
`key`, `scroll`, `drag`, `wait`, `windows`, `screen_destroy` and a few more.
Its skill tells it to use a screen for anything it launches for itself, one
application per screen, and to close it when done.

A screen normally opens a mirror window on one of your spare workspaces, 6
to 9 by default. Switch to it with your usual workspace binding to watch the
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
width = 1280          # a new screen is this wide, and so is every screenshot of it
height = 800          # and this tall, 1280x800 being what vision models read best
max_width = 3840      # refuse a wider screen, whichever agent asks for it
max_height = 2160     # refuse a taller one
max_per_session = 4   # screens one agent session may hold at once, 0 lifts the limit
mirror = true         # open the window you watch a screen through, unless the agent says otherwise
notify = true         # tell you, through your desktop notifications, when a screen opens or closes

[workspaces]
agent = [11, 99]      # the screens live here, one workspace each, out of your way
mirror = [6, 9]       # the mirror windows land here, on the first workspace holding nothing

[lifecycle]
safety_timer = "15m"  # close a screen this long after the session that opened it died

[cage]
renderer = "auto"     # auto tries the GPU first and falls back, gles or pixman forces one
```

**The mirror window.** `mirror` is a default, not a rule. With `mirror = true`
every screen opens its window unless the agent has a reason not to, for
instance a long batch you never asked to watch. With `mirror = false` no
screen opens one unless the agent judges that this one is worth showing you.
Agents are told to leave the choice to you unless you said something about
watching. When a screen has no mirror, `hyprcage list` gives the reason in
`mirror_note`, and `wl-mirror <screen>` opens one at any time.

## Compatibility

Arch Linux and derivatives, Hyprland ≥ 0.50 in either configuration mode
(`hyprland.conf` or `hyprland.lua`), any launcher, any desktop shell.
Verified on Hyprland 0.56 with vanilla Arch and with Omarchy, on a real GPU
and under software rendering, at monitor scales 1, 1.6 and 2. The aarch64
build is tested under emulation only.

A screen adds and removes a Hyprland output, which desktop shells watch.
Some of them run their display logic again on every such event, Omarchy
among them, which on a laptop can make a real monitor blink. hyprcage keeps
its own share of that to one output declaration per screen, and puts back
any workspace, focus or cursor position the removal moved, but the rest
belongs to the shell. `~/.local/state/hyprcage/log/restore.log` records
everything that moved under you and whether it was put back.

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
