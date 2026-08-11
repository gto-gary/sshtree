"""sshtui application."""

from __future__ import annotations

import argparse
import os

from textual.app import App

from .screens.host_list import HostListScreen


class SshTuiApp(App[str]):
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
            "Print the selected host's alias instead of connecting "
            "directly. For wrapping in a shell function that runs ssh "
            "itself, so the command shows up in shell history - see "
            "README for the reconnect-with-up-arrow setup."
        ),
    )
    args = parser.parse_args()

    # App.run() blocks until the app exits and the terminal is fully
    # restored, then returns the alias passed to Screen.exit(alias).
    # Only then is it safe to exec ssh into the same terminal.
    alias = SshTuiApp().run()
    if not alias:
        return
    if args.print_only:
        print(alias)
    else:
        os.execvp("ssh", ["ssh", alias])
