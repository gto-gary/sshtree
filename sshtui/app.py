"""sshtui application."""

from __future__ import annotations

import os

from textual.app import App

from .screens.host_list import HostListScreen


class SshTuiApp(App[str]):
    CSS_PATH = "app.css"
    TITLE = "sshtui"

    def on_mount(self) -> None:
        self.push_screen(HostListScreen())


def run() -> None:
    # App.run() blocks until the app exits and the terminal is fully
    # restored, then returns the alias passed to Screen.exit(alias).
    # Only then is it safe to exec ssh into the same terminal.
    alias = SshTuiApp().run()
    if alias:
        os.execvp("ssh", ["ssh", alias])
