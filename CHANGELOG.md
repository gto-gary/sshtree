# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versioning follows [Semantic Versioning](https://semver.org/). While
the project is on a `0.x` version, anything may still change between
minor releases.

## [Unreleased]

## [0.4.3] - 2026-08-16

### Added

- `--config <path>` flag to read and write a different ssh config file
  instead of the default `~/.ssh/config`.
- `R` connects and records the full session to
  `~/Documents/sshtui/<alias>-<timestamp>.log` via `script(1)`, both when
  run directly and through the shell-integration wrapper.

### Fixed

- `app.css` was missing from built wheels (any non-editable install -
  `pip install`, `uv tool install` without `--editable`, etc.), causing a
  `StylesheetError` at launch. Only the documented `--editable` install
  worked, since it reads straight from the source tree instead of a built
  wheel.

## [0.4.2] - 2026-08-15

### Fixed

- Fixed terminal size detection in terminal emulators without in-band resize escape query support (such as macOS Terminal.app and GNOME Terminal). When run via the shell wrapper (`output=$(command sshtui --print-only)`), `sys.stdout` is a pipe, causing standard size queries to fail and fall back to 80x24 in the top-left corner. `sshtui` now fallback-inspects `sys.stderr`, `sys.stdin`, and `/dev/tty` so the UI opens across the full terminal window.
- Added support for `$XDG_CONFIG_HOME` environment variable when resolving config backup (`$XDG_CONFIG_HOME/sshtui/backups/`) and history paths (defaulting to `~/.config/sshtui/`).
- Fixed column header (`#column-header`) text wrapping when shrinking terminal window width by forcing single-line height (`height: 1`) and disabling line wrap (`text-wrap: nowrap`). Synchronized header horizontal scroll offset (`styles.offset`) with host tree horizontal scrolling (`scroll_x`).

## [0.4.1] - 2026-08-15

### Added

- Live host-count/reachability summary in the header row, right-aligned
  opposite the banner. Shows total hosts, up/down counts, and a
  "checking" count while reachability results are still coming in;
  narrows to match whenever the search filter is active.
- Search filter gained `status:down` / `status:up` / `status:unknown`
  syntax (with `unreachable`/`reachable`/`checking` as synonyms) to
  filter the host list to a specific reachability state, mirroring the
  existing `extra:yes`/`extra:no` exact-match pattern.

### Changed

- Replaced the multi-line ASCII banner with a single-line bold wordmark
  (🔑 `sshtui` + dimmed version), freeing up the ~5 rows of vertical
  space the art used to take. The reachability summary sits right-
  aligned on the same row instead of stacking underneath it, and the
  row gets a 1-column padding on each edge so neither the wordmark nor
  the stats run flush to the terminal border.

## [0.4.0] - 2026-08-15

### Added

- Drop into an interactive `sftp` prompt on the selected host (`f`).
  Same clean-exit-then-exec pattern as connecting.
- Copy a file to/from the selected host via `scp` (`s`): prompts for a
  local path and remote path, then upload or download. Same
  clean-exit-then-exec pattern as connecting - `scp`'s progress bar and
  any password prompt behave like typing the command yourself. Works
  the same directly or through the shell-integration wrapper.
- Local file/folder picker (**Browse...**) for the scp local path field,
  so the local side doesn't require typing a path by hand: navigate a
  `DirectoryTree` starting at `$HOME`, select a file directly, or use
  **Use this folder** to pick the currently-navigated directory (for a
  download destination). A jump-to-path field handles going *up*, since
  `DirectoryTree` can only navigate down from its starting root.
- `--print-only` mode reworked to print the chosen action as plain
  lines (`connect <alias>`, `sftp <alias>`, or `scp <upload|download>
  <alias> <local> <remote>`) instead of just an alias, so the wrapper
  can run `ssh`, `sftp`, or `scp` afterward and inject the right
  command into shell history for any of them. The real command always
  runs after the picker's own output has been fully captured and
  consumed, so it gets a normal terminal either way - confirmed `scp`'s
  progress bar and an interactive `sftp` prompt both reach the real
  terminal correctly through the wrapper, not just when run directly.

### Fixed

- `ScpScreen` and `FilePickerScreen` had no scroll mechanism at all, so
  on a short terminal (e.g. 80x10) their buttons rendered fully
  off-screen with nothing to scroll them into view - worse than the
  `HostEditScreen` bug fixed earlier, which at least clipped rather
  than fully hid them. Fixed with the same pattern: buttons live
  outside a `VerticalScroll` wrapping the rest of the content, so they
  stay pinned and visible regardless of terminal height. Verified at
  80x10 with region-visibility checks and a full Tab-navigation trace.
- Error messages in `ScpScreen` and `FilePickerScreen` (e.g. "Local
  file not found") persisted on screen even after the underlying field
  was fixed, until the next validation attempt - misleadingly implying
  a now-valid path was still wrong. Both now clear their error the
  moment the relevant field changes.
- The `VerticalScroll` fix above introduced its own regression:
  `#picker-tree`'s `height: 1fr` meant something different once nested
  inside a scrolling container (which sizes to content, not "remaining
  space") than it did as a direct child of a fixed-size dialog - the
  tree collapsed to a near-useless 2 rows tall at *any* terminal size,
  including a normal 80x24, and at small heights that sliver fell
  outside the visible viewport entirely. Fixed by giving the tree an
  explicit height (20 rows) instead, so it's consistently usable and
  the surrounding `VerticalScroll` handles genuinely-too-small
  terminals by actually scrolling, rather than squeezing the tree into
  uselessness. Verified across a size matrix (80x24 down to 80x8) that
  the tree keeps its full height and the scroll container reports
  real, positive `max_scroll_y` when content doesn't fit.

## [0.3.0] - 2026-08-13

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

- The add/edit host dialog had no scroll mechanism, so on a short
  terminal the Save/Cancel buttons could be clipped off with no way to
  reach them. The button row now lives outside the scrollable fields
  area and stays pinned and visible regardless of terminal height; Tab
  navigation reaches it without needing to scroll at all. Verified down
  to an 80x10 terminal with a host that has 7 extra parameter rows -
  full Tab sequence from the alias field through every row to Save
  confirmed reachable.
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
- Connecting when `ssh` isn't on `PATH` raised a raw `FileNotFoundError`
  traceback instead of a clean error message.
- An alias with a leading/trailing/doubled `--` (e.g. `srv--`) produced
  a blank-looking row (empty leaf display name) instead of falling back
  to the `other` group with its literal alias shown.
- `Port` can legitimately be a named service (e.g. `Port ssh`, `Port
  http`), resolved by `ssh` itself via `/etc/services` rather than being
  a plain number. The reachability check previously fell back to the
  default port (22) for any non-numeric value - coincidentally correct
  for `Port ssh` but silently wrong for e.g. `Port http` (would check 22
  instead of 80). Now resolved via `socket.getservbyname()`, the same
  mechanism `ssh` uses; verified against `ssh -G` output for several
  named ports.

### Removed

- `history.sort_aliases()` - dead code left over from before the switch
  to alphabetical sorting; nothing called it anymore. `history.py` still
  records usage (`record_use()`), just isn't read back for sort order.

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
