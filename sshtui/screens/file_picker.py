"""Modal for picking a local file or folder, so scp's local path doesn't
require typing the full path by hand."""

from __future__ import annotations

from pathlib import Path

from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical
from textual.screen import ModalScreen
from textual.widgets import Button, DirectoryTree, Input, Static


class FilePickerScreen(ModalScreen[str | None]):
    def __init__(self, start_path: str | None = None) -> None:
        super().__init__()
        self.start_path = start_path or str(Path.home())
        self._current_dir = self.start_path

    def compose(self) -> ComposeResult:
        with Vertical(id="picker-dialog"):
            yield Static(
                "Select a file, or navigate to a folder and use it directly",
                id="picker-title",
            )
            with Horizontal(id="picker-jump"):
                yield Input(value=self.start_path, id="picker-path-input")
                yield Button("Go", id="jump")
            yield DirectoryTree(self.start_path, id="picker-tree")
            yield Static("", id="picker-error")
            with Horizontal(id="picker-buttons"):
                yield Button("Use this folder", id="use-folder")
                yield Button("Cancel", id="cancel", variant="error")

    def _show_error(self, message: str) -> None:
        self.query_one("#picker-error", Static).update(message)

    def on_directory_tree_file_selected(self, event: DirectoryTree.FileSelected) -> None:
        self.dismiss(str(event.path))

    def on_directory_tree_directory_selected(
        self, event: DirectoryTree.DirectorySelected
    ) -> None:
        self._current_dir = str(event.path)

    async def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "cancel":
            self.dismiss(None)
        elif event.button.id == "use-folder":
            self.dismiss(self._current_dir)
        elif event.button.id == "jump":
            await self._jump()

    async def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.input.id == "picker-path-input":
            await self._jump()

    async def _jump(self) -> None:
        """DirectoryTree can only browse *down* from whatever root it was
        given - there's no built-in way to go up past it. Typing a new
        absolute path here and swapping in a freshly-rooted tree is how
        you reach anywhere else (e.g. starting at $HOME but needing /tmp)."""
        target = self.query_one("#picker-path-input", Input).value.strip()
        path = Path(target).expanduser()
        if not path.is_dir():
            self._show_error(f"Not a directory: {target}")
            return
        self._current_dir = str(path)
        old_tree = self.query_one("#picker-tree", DirectoryTree)
        await old_tree.remove()
        await self.query_one("#picker-dialog", Vertical).mount(
            DirectoryTree(str(path), id="picker-tree"), before="#picker-error"
        )
