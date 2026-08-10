# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versioning follows [Semantic Versioning](https://semver.org/). While
the project is on a `0.x` version, anything may still change between
minor releases.

## [Unreleased]

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
