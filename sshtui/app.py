"""sshtui application."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

from textual.app import App

from .actions import ConnectAction, ScpAction, SftpAction
from .screens.host_list import HostListScreen


class SshTuiApp(App["ConnectAction | SftpAction | ScpAction"]):
    CSS_PATH = "app.css"
    TITLE = "sshtui"

    def on_mount(self) -> None:
        self.push_screen(HostListScreen())


def run() -> None:
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
    args = parser.parse_args()

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
