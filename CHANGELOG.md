# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versioning follows [Semantic Versioning](https://semver.org/). While
the project is on a `0.x` version, anything may still change between
minor releases.

This changelog covers the Go rewrite only, starting fresh from its own
`0.1.0`. For the prior Python implementation's history, see
`CHANGELOG.md` on the `python` branch.

The project was later renamed from `sshtui` to `sshtree` (to avoid a name
collision with an unrelated `sshtui` project) and this changelog restarts
again at `0.1.0` under the new name. For history prior to the rename, see
the `sshtui` tags up to `v0.6.0`.

## [Unreleased]

## [0.1.0] - 2026-10-03

### Changed

- Renamed the project from `sshtui` to `sshtree`: new module path
  (`github.com/gto-gary/sshtree`), binary name, and config directory
  (`$XDG_CONFIG_HOME/sshtree`). No functional changes.

## [0.6.0 (sshtui)] - 2026-10-03

### Added

- MIT license.

### Fixed

- Reachability checks now cap concurrent dials at 40 instead of firing one
  per host with no limit. With a few hundred hosts, the unbounded version
  could exceed macOS's default per-process file descriptor limit (`ulimit
  -n`, commonly 256) — worse, Go's dialer races IPv4/IPv6 per host by
  default, roughly doubling the effective socket count — causing dials to
  fail with "too many open files" and show as red/unreachable even for
  hosts that were actually up.

### Documentation

- README: documented the exact backup path, added a dedicated section
  explaining how session recording works and where logs land, and
  clarified that the two installation methods are alternatives, not
  sequential steps.

## [0.5.0] - 2026-09-07

Initial versioned release of the Go rewrite: feature-complete alongside
the Python version, plus the self-contained `--shell-init` mechanism
that supersedes the old external `shell/*.sh` scripts.
