# sshtui

A terminal UI for browsing, connecting to, and editing hosts in `~/.ssh/config`.
Built with [Textual](https://github.com/Textualize/textual) and
[sshconf](https://github.com/sorend/sshconf).

Instead of hand-editing `~/.ssh/config` and running `ssh <alias>` from memory,
`sshtui` gives you a live, grouped, searchable list of your hosts with
one-key connect, add, edit, and delete — while leaving the rest of your file
(comments, spacing, quirky formatting) untouched.

## Features

- **Grouped, tabular host list** — hosts nest by `--` in their alias
  (`srv--nas--x` → group `srv` > subgroup `nas` > host `x`), sorted
  alphabetically, with status/alias/Hostname/Port/User/Extra columns.
- **Connect with `Enter`** — execs `ssh <alias>` in the same terminal, same
  as typing it yourself; optional shell integration makes the up arrow
  recall it afterward.
- **Drop into `sftp` with `f`** — opens an interactive `sftp` prompt on the
  selected host.
- **Copy a file with `s`** — upload or download via `scp`, with a local
  file/folder picker for the local path.
- **Live reachability check** — background TCP status dot per host: 🟢 up,
  🔴 down, ⚪ checking; refreshable on demand with `r`.
- **Search / filter (`/`)** — live filtering by alias/hostname/user/port,
  plus `extra:` and `status:` exact-match filters.
- **Add / edit / delete / clone hosts** — form-based editing with a
  dropdown of common `ssh_config` directives, or **Custom...** for
  anything else.
- **Non-intrusive edits** — powered by `sshconf`; comments, formatting,
  and untouched fields are preserved on save.
- **Automatic backup** — snapshots `~/.ssh/config` before the first write
  in any run.

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

This installs an isolated `sshtui` command onto your `PATH`
(`~/.local/bin/sshtui`). `--editable` means it runs directly against this
source tree, so edits take effect immediately with no reinstall needed.

Make sure `~/.local/bin` is on your `PATH` (add
`export PATH="$HOME/.local/bin:$PATH"` to your shell rc file if not).

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

Run `sshtui` from any shell. The banner at the top shows the app name and
version, plus a live host count and reachability breakdown
(up/down/checking) that narrows to match your search filter.

By default it reads and writes `~/.ssh/config`; use `--config` to point it
at a different file instead:

```sh
sshtui --config ~/.ssh/config.work
```

| Key | Action |
|---|---|
| `↑` / `↓` | Move selection |
| `Enter` | Connect via `ssh` to the selected host |
| `a` | Add a new host |
| `e` | Edit the selected host |
| `c` | Clone the selected host into a new one |
| `f` | Drop into an `sftp` prompt on the selected host |
| `s` | Copy a file to/from the selected host via `scp` |
| `d` | Delete the selected host (asks for confirmation) |
| `r` | Refresh reachability checks for all hosts |
| `/` | Focus the search box |
| `Escape` | Clear the search filter and return focus to the list |
| `q` | Quit |

### Reconnecting with the up arrow

Since `sshtui` execs `ssh`/`sftp`/`scp` directly, your shell history only
ever shows "ran `sshtui`", so the up arrow just reopens the picker instead
of recalling the actual command. To make it recall the real command
instead, add this to your shell rc file (adjusting the path to wherever
you cloned this repo) and reload your shell:

```sh
# ~/.zshrc (or ~/.bashrc, sourcing shell/sshtui.bash instead)
source /path/to/sshtui/shell/sshtui.zsh
```

Use `command sshtui` to bypass it if you ever need the plain installed
command directly.

### Searching

- `/` focuses the search box; type to filter live against alias,
  `Hostname`, `User`, and `Port`. Groups with no matches are hidden.
- `Enter` jumps back into the filtered list; `Escape` clears the filter.
- `extra:yes` / `extra:no` — filter by whether a host has directives
  beyond Hostname/User/Port (the `Extra` column).
- `status:down` / `status:up` / `status:unknown` — filter by reachability
  (also accepts `unreachable`/`reachable`/`checking` as synonyms).
- Group headers (`net`, `srv`, `lab`, `other`) can be expanded/collapsed
  with `Enter` or `Space` while highlighted.

### Adding or editing a host

- Dedicated `Hostname`, `User`, `Port` fields.
- Repeatable rows for any other `ssh_config` option — a directive dropdown
  (or **Custom...**) plus a value field.
- **+ Add parameter** / **✕** — add or remove a row.
- **Save** / **Cancel** — write to `~/.ssh/config`, or discard.

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
| `$XDG_CONFIG_HOME/sshtui/history.json` | `{alias: {count, last_used}}` — drives recent/frequent host tracking (defaults to `~/.config/sshtui/history.json`). |
| `$XDG_CONFIG_HOME/sshtui/backups/config-<timestamp>` | Snapshot of `~/.ssh/config` taken before the first write each run (defaults to `~/.config/sshtui/backups/`). |

## Known limitations

- **Doesn't follow `Include` directives** — only one file is read and
  written per run, so hosts pulled in via `Include` aren't visible unless
  you point `--config` directly at that file instead - there's no unified
  view across multiple files.
- **Grouping is alias-based** — grouping is purely a string convention
  (every `--`-separated segment of the alias becomes a nesting level), not
  a configurable taxonomy.
- **No Windows support** — relies on `os.execvp` and POSIX terminal
  semantics.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for running from source and testing
notes.
