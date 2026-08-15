"""Modal for copying a file to/from the selected host via scp."""

from __future__ import annotations

from pathlib import Path

from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical
from textual.screen import ModalScreen
from textual.widgets import Button, Input, Label, Static

from ..actions import ScpAction
from .file_picker import FilePickerScreen


class ScpScreen(ModalScreen[ScpAction | None]):
    def __init__(self, alias: str) -> None:
        super().__init__()
        self.alias = alias

    def compose(self) -> ComposeResult:
        with Vertical(id="scp-dialog"):
            yield Static(f"Copy file: {self.alias}", id="scp-title")
            yield Label("Local path")
            with Horizontal(id="scp-local-row"):
                yield Input(placeholder="/local/path/to/file", id="scp-local")
                yield Button("Browse...", id="browse")
            yield Label("Remote path")
            yield Input(placeholder="/remote/path/to/file", id="scp-remote")
            yield Static("", id="scp-error")
            with Horizontal(id="scp-buttons"):
                yield Button("Upload →", id="upload", variant="success")
                yield Button("← Download", id="download", variant="primary")
                yield Button("Cancel", id="cancel", variant="error")

    def _show_error(self, message: str) -> None:
        self.query_one("#scp-error", Static).update(message)

    @staticmethod
    def _start_dir_for_picker(current_value: str) -> str | None:
        """DirectoryTree needs a directory to root at. The local field
        might already hold a full file path (typed by hand or from an
        earlier browse) rather than a directory, so fall back to its
        parent; fall back further to the default (None -> $HOME in
        FilePickerScreen) if nothing usable is there."""
        if not current_value:
            return None
        path = Path(current_value).expanduser()
        if path.is_dir():
            return str(path)
        if path.parent.is_dir():
            return str(path.parent)
        return None

    async def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "cancel":
            self.dismiss(None)
            return
        if event.button.id == "browse":
            local_input = self.query_one("#scp-local", Input)
            start = self._start_dir_for_picker(local_input.value.strip())
            picked = await self.app.push_screen_wait(FilePickerScreen(start))
            if picked is not None:
                local_input.value = picked
            return
        if event.button.id not in ("upload", "download"):
            return
        local = self.query_one("#scp-local", Input).value.strip()
        remote = self.query_one("#scp-remote", Input).value.strip()
        if not local:
            self._show_error("Local path cannot be empty.")
            return
        if not remote:
            self._show_error("Remote path cannot be empty.")
            return
        upload = event.button.id == "upload"
        # Only the local path can be checked from here - existence of the
        # remote path depends on the remote host, which we're not
        # connected to yet. For an upload the local path is the source,
        # so catching a typo now beats scp failing after you've already
        # picked a direction and hit go.
        if upload and not Path(local).expanduser().exists():
            self._show_error(f"Local file not found: {local}")
            return
        self.dismiss(ScpAction(self.alias, local, remote, upload))
