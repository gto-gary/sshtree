# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versioning follows [Semantic Versioning](https://semver.org/). While
the project is on a `0.x` version, anything may still change between
minor releases.

## [Unreleased]

### Added

- `--print-only` CLI flag: prints the selected host's alias instead of
  connecting directly, for use by a wrapping shell function.
- Optional shell integration (`shell/sshtui.zsh`, `shell/sshtui.bash`):
  after an SSH session ends, the up arrow recalls `ssh <alias>` and lets
  you reconnect directly without reopening the picker.
- Directives that legitimately repeat (e.g. multiple `IdentityFile`
  entries) are now fully supported end to end: the edit form renders one
  row per value instead of collapsing them, and saving writes them back
  as separate lines in the order shown.

### Fixed

- A host with any directive listed twice (e.g. `Port` accidentally
  duplicated) crashed the app - `sshconf` returns repeated directives as
  a list, and several places (`check_reachability`, row rendering,
  search, column widths) assumed a plain string. All now handle both.
- Row labels are built via Rich markup string interpolation; a
  Hostname/User value containing `[...]`-bracket syntax was interpreted
  as styling instead of shown literally. Values are now escaped before
  interpolation.
- Editing a host that already had a multi-value directive (e.g. two
  `IdentityFile` lines) and saving would silently collapse them into one
  invalid comma-joined line, breaking that directive for `ssh` with no
  error shown. Fixed by the same multi-row support noted above.
- `sshconf.set()` reassigns multi-value lists via `list.pop()`, which
  reverses order on disk - significant for order-sensitive directives
  like `IdentityFile`, where `ssh` tries entries in the listed order.
  `config.update_host()` now pre-reverses list values to cancel that out.
- Adding two parameter rows with the same directive silently kept only
  the last one (dict overwrite); both are now correctly preserved as a
  multi-value directive.

## [0.2.0] - 2026-08-10

### Added

- Multi-level nested grouping: any number of `--` segments in an alias
  becomes a nesting level (`srv--nas--host` → `srv` > `nas` > `host`),
  instead of only the first `--` being recognized.
- Search filter now also matches on Port, and gained `extra:yes` /
  `extra:no` syntax to filter by whether a host has directives beyond
  Hostname/User/Port.

### Changed

- Hosts within each group now sort alphabetically instead of by
  recency/frequency of use (groups themselves were already alphabetical).
- README: Features section restructured into scannable headline +
  sub-bullet format; the grouping explanation reworded in plain language;
  implementation-detail deep-dives (terminal-handoff timing, `Key=Value`
  normalization) moved out of the README and kept only as code comments;
  status dots shown as colored emoji so they render in color on
  GitLab/GitHub instead of plain uncolored text; real device hostnames
  replaced with generic examples; "Reachability is TCP-only" removed from
  Known Limitations as it describes intended behavior, not a shortcoming.
- Stopped tracking build artifacts (`*.egg-info/`, `__pycache__/`) in git;
  added `.gitignore`.

## [0.1.0] - 2026-08-09

Initial release.

### Added

- Grouped host list read from `~/.ssh/config`, split into collapsible
  sections by alias prefix (`net--` / `srv--` / `lab--` / `other`).
- Tabular row layout: status, alias, Hostname, Port (defaults to `22`
  when unset), User, and an Extra column flagging hosts with directives
  beyond Hostname/User/Port.
- Connect via `Enter` — execs `ssh <alias>` in the same terminal after a
  clean app exit, landing you in a normal interactive SSH session.
- Live TCP reachability check per host as a background worker, on startup
  and on demand with `r`.
- Add / Edit / Delete host forms, with dedicated Hostname/User/Port
  fields plus a repeatable key-value row editor for any other
  `ssh_config` directive.
- Directive dropdown covering ~48 common `ssh_config` options
  (`ProxyJump`, `IdentityFile`, `KexAlgorithms`, etc.) with a
  **Custom...** fallback for anything else.
- Clone host (`c`) — duplicates an existing host's settings into a new
  entry with a unique suggested `<alias>-copy` name, with inline
  duplicate-alias validation instead of crashing.
- Live search/filter (`/`), matching alias/hostname/user; `Escape`
  clears it.
- Non-intrusive editing via `sshconf` — preserves comments, blank lines,
  and per-host formatting; only fields that actually changed are
  rewritten.
- Automatic backup of `~/.ssh/config` before the first write each
  session.
- ASCII banner with the version number read dynamically from installed
  package metadata.
- Packaged for distribution via `uv tool install --editable`, with a
  proper console-script entry point.

### Fixed

- Terminal-handoff timing: connecting could hang behind a still-raw
  terminal because `ssh` was exec'd inside the key-press handler, before
  Textual had released the terminal. Now the app exits cleanly first,
  and only then execs `ssh`.
- `q` (quit) did nothing — Textual resolves a key binding's action on
  whichever widget's `BINDINGS` matched, not the App, so a dedicated
  `action_quit` had to be added to the screen itself.
- `r` (refresh) reachability checks were running correctly but gave no
  visible feedback when a host's status didn't change, making it look
  broken; rows now flash to "checking" immediately so a refresh is
  always visible.
- `Key=Value` syntax (as opposed to `Key value`) in `ssh_config` was
  silently invisible to the underlying parser and would be orphaned if
  that host were ever deleted; normalized on load so every directive is
  visible and safely editable regardless of which syntax was used.
- Editing one field (e.g. Port) was silently reformatting other
  untouched lines (e.g. User) on hosts with non-uniform indentation,
  because every core field was always resubmitted on save; only
  genuinely-changed values are written now.
- Default keyboard focus landed on the search box instead of the host
  list on startup, which would have made every keybinding (`a`/`e`/`d`/
  `r`/`q`) type into the search box instead of firing until the tree was
  clicked.
