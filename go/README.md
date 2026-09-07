# sshtui (Go)

A terminal UI for browsing, connecting to, and editing hosts in
`~/.ssh/config`. This is a from-scratch Go rewrite of the original Python
([Textual](https://github.com/Textualize/textual)-based) `sshtui`, built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles), and
[Lip Gloss](https://github.com/charmbracelet/lipgloss). It's a drop-in
replacement: same features, same keybindings, same `~/.ssh/config`
read/write behavior, same shell-integration protocol — just a single static
binary instead of a Python/uv install.

Feature-complete as of this rewrite: grouped host list, live reachability
checks, search/filter, add/edit/clone/delete, scp upload/download with a
file picker, connect/record/sftp, and the `--print-only` shell-integration
mode. See [Differences from the Python version](#differences-from-the-python-version)
for the handful of deliberate simplifications.

## Requirements

- Go 1.24.2+ to build from source (nothing extra needed to just run the
  binary once built)
- macOS or Linux (anywhere `ssh`/`sftp`/`scp` and a terminal are available;
  no Windows support — relies on `syscall.Exec` and POSIX terminal
  semantics, same limitation as the Python version)

## Installation

From this directory:

```sh
go build -o sshtui ./cmd/sshtui
```

Then put the resulting `sshtui` binary somewhere on your `PATH` (e.g.
`~/.local/bin/`). Or install it directly into your Go bin directory:

```sh
go install ./cmd/sshtui
```

No venv, no `uv`, no runtime dependency — it's a single static binary.

### Updating

Pull new code, rebuild, replace the binary. That's the whole story.

### Uninstalling

Delete the binary from wherever you put it.

## Usage

Run `sshtui` from any shell. By default it reads and writes
`~/.ssh/config`; use `--config` to point it at a different file instead:

```sh
sshtui --config ~/.ssh/config.work
```

| Key | Action |
|---|---|
| `↑`/`k`, `↓`/`j` | Move selection |
| `Enter` / `Space` | Connect via `ssh` (on a host) or expand/collapse (on a group) |
| `R` | Connect and record the session to `~/Documents/sshtui/` |
| `f` | Drop into an `sftp` prompt on the selected host |
| `s` | Copy a file to/from the selected host via `scp` |
| `a` | Add a new host |
| `e` | Edit the selected host |
| `c` | Clone the selected host into a new one |
| `d` | Delete the selected host (asks for confirmation) |
| `r` | Refresh reachability checks for all hosts |
| `/` | Focus the search box |
| `Escape` | Clear the search filter and return focus to the list |
| `q` / `Ctrl+C` | Quit |

### Searching

Same mini-language as the Python version: plain text substring-matches
against alias/hostname/user/port; `extra:yes`/`extra:no` filters by whether
a host has directives beyond Hostname/User/Port; `status:up`/`status:down`/
`status:unknown` (or `reachable`/`unreachable`/`checking`) filters by live
reachability.

### Adding or editing a host

Dedicated Hostname/User/Port fields, plus a scrollable list of arbitrary
`ssh_config` directive rows. `Tab`/`Shift+Tab` moves between fields,
`Ctrl+N` adds a row, `Ctrl+D` removes the focused row, `Ctrl+S` saves,
`Esc` cancels.

### Copying a file

`s` opens the scp form: local path, remote path, `Ctrl+F` to browse for the
local file (a real up-and-down file picker — not limited to browsing below
a starting directory), `Ctrl+U` to upload, `Ctrl+G` to download, `Esc` to
cancel.

### Reconnecting with the up arrow

Same shell integration as the Python version — the wrapper scripts are
unchanged and work with either binary, since they just shell out to
`sshtui --print-only` and parse its stdout:

```sh
# ~/.zshrc (or ~/.bashrc, sourcing shell/sshtui.bash instead)
source /path/to/sshtui/shell/sshtui.zsh
```

Use `command sshtui` to bypass it if you need the plain installed command
directly.

## How it works

### File layout

```
go/
├── go.mod
├── cmd/sshtui/          # entry point, flag parsing, exec dispatch
│   ├── main.go
│   └── dispatch.go      # syscall.Exec into ssh/sftp/scp/script
└── internal/
    ├── config/          # format-preserving ~/.ssh/config parser/writer
    ├── actions/         # Connect/Sftp/Scp action types
    ├── history/         # recent/frequent-use tracking (unused for display, kept for parity)
    ├── reachability/     # concurrent TCP reachability checks
    └── ui/               # Bubble Tea model: host list + all child screens
```

### Data files

Same locations and formats as the Python version:

| Path | Purpose |
|---|---|
| `~/.ssh/config` | The source of truth. Read on every screen refresh, written on every save. |
| `$XDG_CONFIG_HOME/sshtui/history.json` | `{alias: {count, last_used}}` (defaults to `~/.config/sshtui/history.json`). |
| `$XDG_CONFIG_HOME/sshtui/backups/config-<timestamp>` | Snapshot of `~/.ssh/config` taken before the first write each run (defaults to `~/.config/sshtui/backups/`). |
| `~/Documents/sshtui/<alias>-<timestamp>.log` | Recorded session logs (from `R`). |

### The config parser

The highest-risk piece of this port: `internal/config` is a hand-rolled,
format-preserving parser/writer (no existing Go library does this well).
Comments, blank lines, indentation, and any directive not explicitly
touched by an edit round-trip byte-for-byte. See
`internal/config/config_test.go` for the golden-file test suite this is
verified against — that's the actual safety net for a bug class that could
otherwise corrupt a real `~/.ssh/config`.

## Known limitations

Same as the Python version:

- **Doesn't follow `Include` directives** — only one file is read/written
  per run; point `--config` directly at an included file if you need to
  edit it.
- **Grouping is alias-based** — every `--`-separated segment of the alias
  becomes a nesting level; it's a string convention, not a configurable
  taxonomy.
- **No Windows support.**

## Differences from the Python version

A few deliberate simplifications made during the rewrite, none of which
lose core functionality:

- **No directive-name dropdown/autocomplete** in the edit form — directive
  names are free-text instead of picked from a curated list with a
  "Custom..." escape hatch. Typing `ProxyJump` works exactly the same as
  selecting it would have; you just don't get autocomplete.
- **No "jump to an arbitrary path" in the file picker** — Python needed
  this because Textual's `DirectoryTree` can only browse *downward* from
  its start point. `bubbles/filepicker` navigates both up and down freely,
  so the workaround wasn't needed.
- A handful of small **format-preservation improvements** over the Python
  version's underlying `sshconf` library: an edited directive's original
  indentation and case are preserved (sshconf always re-indents and
  lowercases on edit), a trailing inline comment on an edited line survives
  the edit, and a rename preserves any comment on the `Host` line itself
  (sshconf drops it). None of these are behavior changes you'd notice
  day-to-day — just less incidental reformatting of your config file.
