# sshtui

A terminal UI for browsing, connecting to, and editing hosts in `~/.ssh/config`.
Built with [Textual](https://github.com/Textualize/textual) and
[sshconf](https://github.com/sorend/sshconf).

Instead of hand-editing `~/.ssh/config` and running `ssh <alias>` from memory,
`sshtui` gives you a live, grouped, searchable list of your hosts with
one-key connect, add, edit, and delete — while leaving the rest of your file
(comments, spacing, quirky formatting) untouched.

## Features

- **Grouped, tabular host list**
  - Each `--` in a host's alias adds another level of nesting, so groups
    automatically match however you've named your hosts:
    - `net--examplerouter1` → group `net`, host `examplerouter1`
    - `srv--nas--examplenas` → group `srv` > subgroup `nas`, host `examplenas`
    - No `--` at all → falls into a single `other` group
  - Columns: status, alias, Hostname, Port (shows `22` when unset), User,
    and Extra (`Yes`/`No` for directives beyond Hostname/User/Port).
  - Column widths auto-adjust to the longest visible value each render.
  - Groups and the hosts within them are sorted alphabetically.

- **Connect with `Enter`**
  - Exits the TUI cleanly, then execs `ssh <alias>` in the same terminal —
    same experience as typing the command yourself, including password
    prompts.
  - Optional shell integration (`shell/sshtui.zsh` / `.bash`) makes the up
    arrow recall `ssh <alias>` after the session ends, so you can
    reconnect directly without reopening the picker.

- **Copy a file with `s`**
  - Prompts for a local path and remote path, then upload or download —
    same clean-exit-then-exec pattern as connecting, so `scp`'s progress
    bar and any password prompt behave exactly like typing the command
    yourself.
  - Not available when running through the shell-integration wrapper
    (its stdout capture is designed for the reconnect trick, not for
    scp's interactive output) — use `command sshtui` directly for this.

- **Live reachability check**
  - Status dot per row: 🟢 up, 🔴 down/unreachable, ⚪ checking.
  - Background TCP connect check against the host's port (default `22`,
    or its configured `Port`).
  - Runs on startup and on demand with `r`.

- **Search / filter (`/`)**
  - Matches alias, hostname, user, and port as you type.
  - `extra:yes` / `extra:no` — exact filter on whether a host has
    directives beyond Hostname/User/Port.
  - Re-renders from the already-loaded config, not a fresh disk read per
    keystroke — stays responsive with hundreds of hosts.
  - `Escape` clears the filter and returns focus to the list.

- **Add / edit / delete / clone hosts**
  - Form fields for `Hostname` / `User` / `Port`, plus repeatable rows for
    any other `ssh_config` directive.
  - Directive dropdown covers ~48 common options (`ProxyJump`,
    `IdentityFile`, `KexAlgorithms`, `ServerAliveInterval`, etc.) with a
    **Custom...** fallback for anything else.
  - `c` clones the selected host, pre-filling a unique `<alias>-copy` name
    you can rename before saving.
  - Duplicate-alias saves are rejected inline rather than crashing or
    silently overwriting.

- **Non-intrusive edits**
  - Powered by `sshconf` — comments, blank lines, and per-host formatting
    quirks are preserved.
  - Only fields that actually changed get rewritten, so editing one field
    never silently reformats an untouched line.
  - Directives that legitimately repeat (multiple `IdentityFile` entries
    being the common case) get one row per value in the edit form, and
    are written back as separate lines in the order shown — not
    collapsed into one invalid comma-joined line.


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
| `s` | Copy a file to/from the selected host via `scp` |
| `d` | Delete the selected host (asks for confirmation) |
| `r` | Refresh reachability checks for all hosts |
| `/` | Focus the search box |
| `Escape` | Clear the search filter and return focus to the list |
| `q` | Quit |

### Reconnecting with the up arrow

By default, `sshtui` connects by exec'ing `ssh <alias>` directly, which
means your shell only ever sees "ran `sshtui`" in its history - not the
actual `ssh` command - so pressing the up arrow after a session ends (from
quitting, an idle timeout, or a dropped connection) just recalls `sshtui`
again, reopening the picker.

To make the up arrow recall `ssh <alias>` instead - so you can reconnect
directly without going back through the picker - set up the shell function:

1. Add this line to your shell rc file, adjusting the path to wherever
   you cloned this repo:

   ```sh
   # ~/.zshrc (or ~/.bashrc, sourcing shell/sshtui.bash instead)
   source /path/to/sshtui/shell/sshtui.zsh
   ```

2. Reload it - `source ~/.zshrc`, or just open a new terminal.

This defines a `sshtui` shell function that shadows the installed command:
it runs the picker in `--print-only` mode (which prints the chosen alias
instead of connecting directly), injects `ssh <alias>` into your shell's
history, then connects. The function falls through to nothing on quit
without selecting a host. Use `command sshtui` if you ever need to bypass
the function and reach the plain installed command directly.

No reinstall is needed for this - it's a plain shell rc change, unrelated
to how `sshtui` itself is installed.

Because this mode captures the picker's output to build the `ssh`
command, copying a file (`s`) is disabled while running through the
wrapper - `scp`'s progress bar and any prompts would be captured too
instead of reaching your terminal, and the wrapper wouldn't know to skip
its own `ssh` step afterward. Selecting `s` under the wrapper shows a
warning instead of proceeding; use `command sshtui` directly to copy
files.

### Searching

Press `/` to jump into the search box (from anywhere except while already
typing in it), then type to filter live — matches are checked against the
alias, `Hostname`, `User`, and `Port` of every host. Groups with no matches
are hidden entirely. Press `Enter` to jump back into the filtered list, or
`Escape` at any point to clear the filter and see everything again.

Typing `extra:yes` filters to only hosts with directives beyond
Hostname/User/Port (the `Extra` column); `extra:no` filters to hosts
without any. This is an exact flag match rather than a substring search,
since "has extra params" is a yes/no property, not text to search within.

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

## Known limitations

- **Single file only** — `Include` directives in `~/.ssh/config` (for
  pulling in other config files) are not followed; only the top-level file
  is read and written.
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
