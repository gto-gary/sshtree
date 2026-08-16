# Contributing

## Running from source

Run the app straight from source without touching the installed tool:

```sh
uv run sshtui
```

`uv run` auto-detects `pyproject.toml`, creates/reuses a local `.venv` in
this directory, and installs the declared dependencies — no flags needed.
This `.venv` is separate from the one `uv tool install` manages under
`~/.local/share/uv/tools/sshtui/`; either one reflects live edits to
`sshtui/` immediately since both ultimately run this same source tree.

## Testing

There is no automated test suite; changes have been verified ad hoc using
Textual's headless `Pilot` testing API (`App.run_test()`) against sandbox
copies of `~/.ssh/config` in `/tmp`, never against the real file directly.
