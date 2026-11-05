# rex-pr-glance

A [Rex](https://www.superlogical.com/rex) plugin that shows the GitHub pull
request of the branch you work on, in a pane beside your terminal. The pane is
[PR Glance](https://github.com/cupsadarius/pr-glance): CI checks, review
decision, stack, comments and review threads, read through your authenticated
`gh`. It is read-only against GitHub.

The pane follows the terminal you work in. When you focus another terminal in
the same window, the pane shows that terminal's checkout within two seconds.
When you focus the pane itself, it keeps the last terminal.

## Prerequisites

- Rex
- `git`
- `gh`, authenticated (`gh auth login`)
- a POSIX shell, `tar`, `gzip` and `sha256sum` or `shasum`, for the installer
- macOS or Linux, amd64 or arm64

## Install

Clone into Rex's configuration directory and install the binary for the
checked-out version:

```sh
git clone https://github.com/cupsadarius/rex-pr-glance ~/.config/rex/rex-pr-glance
sh ~/.config/rex/rex-pr-glance/scripts/install.sh
```

The installer downloads the release that matches `VERSION`, checks its SHA-256
against the release checksums, and installs it as `bin/rex-pr-glance`. Run it
again after every `git pull`. It never falls back to the latest release.

Add to `~/.config/rex/init.lua`:

```lua
require("rex-pr-glance").setup{}
rex.bind("cmd+g", "pr_glance")
```

Then reload:

```sh
rex config reload
```

Rex loads modules from beside the `init.lua` it opens, so if `init.lua` is a
symlink, clone next to the link.

## The `pr_glance` action

One key, three steps:

1. No pane in the window: open it to the right of the focused terminal.
2. Pane open, focus elsewhere: focus the pane.
3. Pane focused: close it.

The action is also in the command palette under "PR Glance".

## Options

```lua
require("rex-pr-glance").setup{
  side = "right",        -- where the pane opens: "right" or "left"
  ratio = 0.65,          -- the part of the split the focused terminal keeps
  label = "pr-glance",   -- the block label that marks the pane
  bin = nil,             -- path to rex-pr-glance; default: bin/ in this clone
}
```

## The binary

`rex-pr-glance view [--from BLOCK]` runs the pane. It needs `REX_SESSION` and
`REX_BLOCK`, which Rex sets in every terminal. `--from` names the terminal to
follow first. It reads Rex state with `rex session inspect --json` and
`rex block call process` every two seconds. It polls GitHub only while its
window is its session's active window, no other block is zoomed over it, and a
Rex app is attached to the session. The Rex app attaches to every session, so
a pane in a session you are not looking at still polls when its window is that
session's active window.

`z` in the pane runs `rex zoom` on the pane. Every other key is PR Glance's; see
its [controls](https://github.com/cupsadarius/pr-glance#controls).

The discussion cache lives in `$XDG_STATE_HOME/rex-pr-glance`, default
`~/.local/state/rex-pr-glance`.

## Remote Rex servers

Rex runs a key-bound action on the host of the session, and the action starts
the binary by its absolute local path. To use the plugin in a session on a
remote Rex server, clone and install it at the same absolute path on that host,
or set `bin` to the path there.
