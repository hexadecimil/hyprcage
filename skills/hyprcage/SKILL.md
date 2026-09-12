---
name: hyprcage
description: REQUIRED before launching any graphical application FOR YOURSELF (testing a UI, taking screenshots, driving an app, computer-use) on a Hyprland desktop. Not for apps the human asked you to open for them: those go on their normal screens. Triggers - launch/open/test a GUI app, screenshot of an app, click/type in an app, computer-use, open_application, headless output.
---

# hyprcage: your own screen, never the human's

The human works on workspaces 1–5 with their own mouse and keyboard. Anything
you launch for yourself goes on a **hyprcage screen**: a virtual output with its
own compositor and seat. The human can watch it through a mirror window on one
of their spare workspaces (6–9 by default) and is never interrupted.

## When

| Situation | hyprcage? |
|---|---|
| You launch an app to test, measure, capture or drive it | **yes** |
| The human says "open X", "show me X" | **no**, launch it normally |
| Unsure | ask in one line |

## First use on a machine

The plugin installs its own binary on the first start. If `screen_create`
fails with `cage_missing`, the machine lacks cage, the agent's compositor:
tell the human that a password dialog is about to open, call `setup`, then
retry. Nothing else to install.

## How (MCP tools)

1. `screen_create` → **one screen per application** (cage shows one app at a time), 1280x800 by default. Remember its name.
2. `app_launch` with the command. Chromium: `chromium --ozone-platform=wayland --user-data-dir=/tmp/hc-profile --no-first-run <url>`.
3. `screenshot`, then `click` / `type` / `key` / `scroll` / `drag` with **screen pixel coordinates**. Use `wait` (stability or title) instead of sleeping. Ask for `screenshot_after` only when you need to see the result.
4. `screen_destroy` **as soon as you are done**, before handing back to the human. If you keep a screen open between steps, say so.

## Never

- `app &` from a shell, `open_application` from computer-use, or any launch outside `app_launch`: it lands on the human's screen.
- `hyprctl dispatch focus*`, `workspace`, `movecursor`: they touch the human.
- Destroying a screen you did not create (`not_owner`).
- Keeping every screenshot in context: each one costs ~1 300 tokens.
- Launching a browser or Electron app (Discord, Chromium, Firefox…) with the human's profile while theirs is open: single-instance apps hand the launch to the running instance, and the window opens on the human's screen. Always a dedicated profile (`--user-data-dir`), or their instance closed.

## CLI twin

Everything exists as `hyprcage <command>` for a terminal: `create`, `launch`,
`shot`, `click`, `type`, `key`, `destroy`, `list`, `gc`, `doctor`.
