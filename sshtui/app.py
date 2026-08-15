"""sshtui application."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

from textual.app import App

from .actions import ConnectAction, ScpAction
from .screens.host_list import HostListScreen


class SshTuiApp(App["ConnectAction | ScpAction"]):
    CSS_PATH = "app.css"
    TITLE = "sshtui"

    def __init__(self, print_only: bool = False) -> None:
        super().__init__()
        self.print_only = print_only

    def on_mount(self) -> None:
        self.push_screen(HostListScreen())


def run() -> None:
    parser = argparse.ArgumentParser(prog="sshtui")
    parser.add_argument(
        "--print-only",
        action="store_true",
        help=(
            "Print the selected host's alias instead of connecting "
            "directly. For wrapping in a shell function that runs ssh "
            "itself, so the command shows up in shell history - see "
            "README for the reconnect-with-up-arrow setup. Copying a "
            "file (scp) is unavailable in this mode - its progress "
            "output would be captured by the wrapper, not shown."
        ),
    )
    args = parser.parse_args()

    # App.run() blocks until the app exits and the terminal is fully
    # restored, then returns whatever was passed to Screen.exit(...).
    # Only then is it safe to exec ssh/scp into the same terminal.
    result = SshTuiApp(print_only=args.print_only).run()
    if result is None:
        return

    if isinstance(result, ConnectAction):
        if args.print_only:
            print(result.alias)
            return
        try:
            os.execvp("ssh", ["ssh", result.alias])
        except FileNotFoundError:
            print("error: 'ssh' not found on PATH", file=sys.stderr)
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
