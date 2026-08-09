# sshtui

A terminal UI for browsing, connecting to, and editing hosts in `~/.ssh/config`.
Built with [Textual](https://github.com/Textualize/textual) and
[sshconf](https://github.com/sorend/sshconf).

Instead of hand-editing `~/.ssh/config` and running `ssh <alias>` from memory,
`sshtui` gives you a live, grouped, searchable list of your hosts with
one-key connect, add, edit, and delete — while leaving the rest of your file
(comments, spacing, quirky formatting) untouched.

## Features

- **Grouped, tabular host list** — hosts are grouped by `--`-separated
  segments in their alias, nested to whatever depth the name implies:
  `net--edgeucg...` → group `net`; `srv--nas--trunas...` → group `srv` >
  subgroup `nas` > leaf `trunas...`. Aliases without a `--` land in an
  `other` group. Within each group, rows are laid out in aligned
  columns: status, alias, Hostname, Port (shows `22` when not explicitly
  set), User, and an Extra column (`Yes`/`No`) flagging whether the host has
  any directives beyond Hostname/User/Port. Column widths adapt to the
  longest visible value each render.
- **Connect with Enter** — selecting a host exits the TUI cleanly and execs
  `ssh <alias>` in the same terminal, so you land directly in your SSH
  session (including password/passphrase prompts) exactly as if you'd typed
  `ssh <alias>` yourself.
- **Live reachability check** — each row shows a status dot (`●` green =
  up, `●` red = down/unreachable, `○` grey = checking) from a background TCP
  connect check to the host's port (default 22, or its configured `Port`).
  Runs on startup and on demand with `r`.
- **Search / filter** — press `/` to filter the list live as you type,
  matching against alias, hostname, and user. Built for scale: filtering
  re-renders from the already-loaded config (no disk re-read per keystroke),
  so it stays responsive even with hundreds of hosts. `Escape` clears the
  filter and returns focus to the list.
- **Add / edit / delete / clone hosts** — a form with dedicated fields for
  `Hostname` / `User` / `Port`, plus a repeatable parameter row for any other
  `ssh_config` directive. The directive field is a dropdown of ~48 common
  directives (`ProxyJump`, `IdentityFile`, `KexAlgorithms`,
  `ServerAliveInterval`, etc.) with a **Custom...** option for anything not
  listed, so arbitrary directives are always supported. `c` clones the
  selected host into a new one with the same settings, pre-filling a unique
  `<alias>-copy` name you can rename before saving; a duplicate-alias name is
  rejected inline rather than crashing or silently overwriting.
- **Non-intrusive edits** — powered by `sshconf`, which edits `~/.ssh/config`
  in place: existing comments, blank lines, and per-host formatting quirks
  are preserved. Only the specific values you actually change are sent to
  the underlying line-rewrite — the edit form always shows every core field,
  but only ones that differ from their original value get touched, so
  saving one field doesn't silently reformat another untouched line.
- **Recent/frequent sort** — within each group, hosts you've connected to
  most recently (then most often) sort to the top.
- **Automatic backup** — before the first write in any run, the current
  `~/.ssh/config` is snapshotted to `~/.config/sshtui/backups/`.

## Requirements

- Python 3.11+
- macOS or Linux (anywhere `ssh` and a terminal are available; not tested on
  Windows)

## Installation

The recommended install method is [`uv`](https://docs.astral.sh/uv/), which
needs no `sudo` and sidesteps distro restrictions like Arch's/Debian's
"externally managed environment" `pip` error.

### 1. Install uv (one-time, per machine)

```sh
curl -LsSf https://astral.sh/uv/install.sh | sh
```

This installs a single static binary to `~/.local/bin`. No `sudo` required.
Works identically on Linux and macOS.

### 2. Install sshtui

From this project directory:

```sh
uv tool install --editable .
```

This creates an isolated virtual environment (managed by `uv`, not by you),
installs `textual` and `sshconf` into it, and links an `sshtui` command onto
your `PATH` (`~/.local/bin/sshtui`). `--editable` means the installed command
runs directly against this source tree — edits to the `sshtui/` package take
effect immediately, no reinstall needed.

Make sure `~/.local/bin` is on your `PATH` (most shells already have this;
if not, add `export PATH="$HOME/.local/bin:$PATH"` to your shell rc file).

### Verify

```sh
sshtui
```

should launch the TUI showing your `~/.ssh/config` hosts.

### Updating / reinstalling

Since the install is editable, pulling new code into this directory is
enough — no reinstall needed. If you ever change `pyproject.toml` (e.g. add
a dependency), re-run:

```sh
uv tool install --editable . --reinstall
```

### Uninstalling

```sh
uv tool uninstall sshtui
```

## Usage

Run `sshtui` from any shell. The top of the screen shows an ASCII banner
with the app name and the installed version, read dynamically from package
metadata (`importlib.metadata`) so it always matches whatever's actually
installed.

| Key | Action |
|---|---|
| `↑` / `↓` | Move selection |
| `Enter` | Connect via `ssh` to the selected host |
| `a` | Add a new host |
| `e` | Edit the selected host |
| `c` | Clone the selected host into a new one |
| `d` | Delete the selected host (asks for confirmation) |
| `r` | Refresh reachability checks for all hosts |
| `/` | Focus the search box |
| `Escape` | Clear the search filter and return focus to the list |
| `q` | Quit |

### Searching

Press `/` to jump into the search box (from anywhere except while already
typing in it), then type to filter live — matches are checked against the
alias, `Hostname`, and `User` of every host. Groups with no matches are
hidden entirely. Press `Enter` to jump back into the filtered list, or
`Escape` at any point to clear the filter and see everything again.

Group headers (`net`, `srv`, `lab`, `other`) can be expanded/collapsed with
`Enter` or `Space` while highlighted.

### Adding or editing a host

The edit form has three dedicated fields — `Hostname`, `User`, `Port` — plus
a list of additional parameter rows. Each row has:

- A **directive dropdown**, pre-populated with common `ssh_config` options.
  Pick **Custom...** to type any directive name not in the list.
- A **value** field.

Click **+ Add parameter** to add another row, or the **✕** button on a row
to remove it. **Save** writes the changes back to `~/.ssh/config`; **Cancel**
discards them.

## How it works

### File layout

```
sshtui/
├── pyproject.toml         # package metadata, deps, console-script entry point
└── sshtui/
    ├── app.py              # Textual App; run() execs ssh after clean exit
    ├── app.css             # styling for modals/dialogs
    ├── __main__.py         # `python -m sshtui` entry point
    ├── config.py           # ~/.ssh/config read/write, wraps sshconf
    ├── history.py          # recent/frequent-use tracking
    ├── reachability.py     # background TCP reachability checks
    └── screens/
        ├── host_list.py    # main screen: tree, keybindings, connect/add/edit/delete
        ├── host_edit.py    # add/edit form + directive dropdown
        └── confirm.py      # yes/no confirmation modal
```

### Data files

| Path | Purpose |
|---|---|
| `~/.ssh/config` | The source of truth. Read on every screen refresh, written on every save. |
| `~/.config/sshtui/history.json` | `{alias: {count, last_used}}` — drives the recent/frequent sort. |
| `~/.config/sshtui/backups/config-<timestamp>` | Snapshot of `~/.ssh/config` taken before the first write each run. |

### Why `Enter` doesn't hang on the password prompt

Textual apps hold the terminal in raw/alt-screen mode while running. If
`ssh` were exec'd immediately inside the key-press handler, its password
prompt would render into a terminal Textual hadn't finished releasing yet,
and appear to hang. Instead, selecting a host calls `self.app.exit(alias)`,
which lets Textual fully tear down and restore the terminal; only *after*
`App.run()` returns (in `app.py`) does `run()` call
`os.execvp("ssh", ["ssh", alias])` — by which point the terminal is back to
normal and `ssh` behaves exactly as if you'd typed the command yourself.

### The `Key=Value` normalization

`ssh_config` allows directives to be written as either `Key value` or
`Key=value`. The `sshconf` library's parser only understands the
space-separated form — a `Key=value` line gets silently detached from its
host block (no host association at all), so it wouldn't show up when
editing, and would be orphaned as a dead line if that host were ever
deleted. `config.py`'s `load()` rewrites any `Key=value` line to `Key value`
before handing the file to `sshconf`, so every directive is visible and
safely editable regardless of which syntax was originally used. This means
the first time you save an edit on a host that used `=` syntax, those lines
will be rewritten to space-separated form in the file — functionally
identical to `ssh`, just a cosmetic change.

## Known limitations

- **Single file only** — `Include` directives in `~/.ssh/config` (for
  pulling in other config files) are not followed; only the top-level file
  is read and written.
- **Reachability is TCP-only** — the status dot reflects whether the host's
  port accepts a TCP connection, not whether SSH authentication would
  actually succeed.
- **Grouping is alias-based** — grouping is purely a string convention
  (every `--`-separated segment of the alias becomes a nesting level), not
  a configurable taxonomy.
- **No Windows support** — relies on `os.execvp` and POSIX terminal
  semantics.

## Development

Run the app straight from source without touching the installed tool:

```sh
uv run sshtui
```

`uv run` auto-detects `pyproject.toml`, creates/reuses a local `.venv` in
this directory, and installs the declared dependencies — no flags needed.
This `.venv` is separate from the one `uv tool install` manages under
`~/.local/share/uv/tools/sshtui/`; either one reflects live edits to
`sshtui/` immediately since both ultimately run this same source tree.

There is no automated test suite; changes have been verified ad hoc using
Textual's headless `Pilot` testing API (`App.run_test()`) against sandbox
copies of `~/.ssh/config` in `/tmp`, never against the real file directly.
