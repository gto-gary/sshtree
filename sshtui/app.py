"""sshtui application."""

from __future__ import annotations

import argparse
import os
import shlex
import sys
import shutil
from pathlib import Path

from textual.app import App

from . import config
from .actions import ConnectAction, ScpAction, SftpAction
from .screens.host_list import HostListScreen


def _ensure_valid_terminal_size() -> None:
    """Ensure Textual gets the true terminal size even when stdout is captured
    by a shell wrapper subshell like `output=$(command sshtui --print-only)`.

    Python's `shutil.get_terminal_size()` only checks `sys.__stdout__`, which
    raises OSError when stdout is a pipe, causing it to fall back to (80, 24).
    Terminals like macOS Terminal.app and GNOME Terminal don't support in-band
    resize queries, staying stuck at 80x24 in the top-left corner.
    """
    orig_get_terminal_size = shutil.get_terminal_size

    def smart_get_terminal_size(fallback: tuple[int, int] = (80, 24)) -> os.terminal_size:
        try:
            cols = int(os.environ.get("COLUMNS", 0))
            lines = int(os.environ.get("LINES", 0))
            if cols > 0 and lines > 0:
                return os.terminal_size((cols, lines))
        except (ValueError, TypeError):
            pass

        for stream in (sys.stdout, sys.stderr, sys.stdin):
            if stream is not None:
                try:
                    return os.get_terminal_size(stream.fileno())
                except (AttributeError, ValueError, OSError):
                    pass

        try:
            with open("/dev/tty") as tty:
                return os.get_terminal_size(tty.fileno())
        except (AttributeError, ValueError, OSError):
            pass

        return orig_get_terminal_size(fallback)

    shutil.get_terminal_size = smart_get_terminal_size


class SshTuiApp(App["ConnectAction | SftpAction | ScpAction"]):
    CSS_PATH = "app.css"
    TITLE = "sshtui"

    def on_mount(self) -> None:
        self.push_screen(HostListScreen())


def run() -> None:
    _ensure_valid_terminal_size()
    parser = argparse.ArgumentParser(prog="sshtui")
    parser.add_argument(
        "--print-only",
        action="store_true",
        help=(
            "Print the chosen action as lines instead of running it "
            "directly, for a wrapping shell function to run itself "
            "afterward (once its own $(...) capture has completed, so "
            "the real command gets a normal terminal) - see README for "
            "the reconnect-with-up-arrow setup."
        ),
    )
    parser.add_argument(
        "--config",
        type=Path,
        default=None,
        help="Path to the ssh config file to use (default: ~/.ssh/config)",
    )
    args = parser.parse_args()

    if args.config is not None:
        config.set_config_path(args.config.expanduser())

    # App.run() blocks until the app exits and the terminal is fully
    # restored, then returns whatever was passed to Screen.exit(...).
    # Only then is it safe to exec ssh/sftp/scp into the same terminal.
    result = SshTuiApp().run()
    if result is None:
        return

    if args.print_only:
        for line in result.print_only_lines():
            print(line)
        return

    if isinstance(result, ConnectAction):
        if result.record_to is not None:
            result.record_to.parent.mkdir(parents=True, exist_ok=True)
            # script's syntax for running a specific command differs by
            # platform: BSD/macOS takes it as trailing argv (no shell
            # involved), util-linux/Linux takes it as a single string via
            # -c that IT shell-execs internally - quote the alias for that
            # one case, since it's the only place in this codebase a value
            # from ~/.ssh/config passes through a shell rather than argv.
            if sys.platform == "darwin":
                cmd = ["script", "-q", str(result.record_to), "ssh", result.alias]
            else:
                cmd = [
                    "script",
                    "-q",
                    "-c",
                    f"ssh {shlex.quote(result.alias)}",
                    str(result.record_to),
                ]
            try:
                os.execvp("script", cmd)
            except FileNotFoundError:
                print("error: 'script' not found on PATH", file=sys.stderr)
                sys.exit(1)
        else:
            try:
                os.execvp("ssh", ["ssh", result.alias])
            except FileNotFoundError:
                print("error: 'ssh' not found on PATH", file=sys.stderr)
                sys.exit(1)
    elif isinstance(result, SftpAction):
        try:
            os.execvp("sftp", ["sftp", result.alias])
        except FileNotFoundError:
            print("error: 'sftp' not found on PATH", file=sys.stderr)
            sys.exit(1)
    elif isinstance(result, ScpAction):
        # Only the local side needs expanding here - "~" isn't expanded
        # by execvp (no shell involved), but the remote side's "~" is
        # meaningless locally anyway; scp/ssh resolve that on the remote
        # host themselves, same as if you'd typed the command by hand.
        local = str(Path(result.local_path).expanduser())
        remote = f"{result.alias}:{result.remote_path}"
        cmd = [local, remote] if result.upload else [remote, local]
        try:
            os.execvp("scp", ["scp", *cmd])
        except FileNotFoundError:
            print("error: 'scp' not found on PATH", file=sys.stderr)
            sys.exit(1)
