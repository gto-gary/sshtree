"""Add/edit screen: fixed core fields plus dynamic key/value rows for
arbitrary SSH directives (KexAlgorithms, ProxyJump, IdentityFile, etc.)."""

from __future__ import annotations

from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import Button, Input, Label, Select, Static

CORE_LABELS = {"hostname": "Hostname", "user": "User", "port": "Port"}

# Common ssh_config Host-block directives, offered as a dropdown so Gary
# doesn't have to remember exact names/casing. Anything else is still
# supported via the "Custom..." option, since ssh_config has ~90 directives
# and hosts may need one that isn't in this shortlist.
COMMON_DIRECTIVES = [
    "AddKeysToAgent", "AddressFamily", "BatchMode", "BindAddress",
    "CanonicalizeHostname", "Ciphers", "Compression", "ConnectTimeout",
    "ControlMaster", "ControlPath", "ControlPersist", "DynamicForward",
    "EscapeChar", "ExitOnForwardFailure", "ForwardAgent", "ForwardX11",
    "GatewayPorts", "HashKnownHosts", "HostKeyAlgorithms", "IdentitiesOnly",
    "IdentityAgent", "IdentityFile", "KbdInteractiveAuthentication",
    "KexAlgorithms", "LocalCommand", "LocalForward", "LogLevel", "MACs",
    "PasswordAuthentication", "PermitLocalCommand", "PreferredAuthentications",
    "ProxyCommand", "ProxyJump", "PubkeyAcceptedKeyTypes", "PubkeyAuthentication",
    "RemoteCommand", "RemoteForward", "RequestTTY", "SendEnv",
    "ServerAliveCountMax", "ServerAliveInterval", "SetEnv", "StrictHostKeyChecking",
    "TCPKeepAlive", "Tunnel", "UserKnownHostsFile", "VisualHostKey",
    "XAuthLocation",
]
_DIRECTIVE_BY_LOWER = {d.lower(): d for d in COMMON_DIRECTIVES}
CUSTOM = "__custom__"


class KeyValueRow(Horizontal):
    def __init__(self, key: str = "", value: str = "") -> None:
        super().__init__(classes="kv-row")
        canonical = _DIRECTIVE_BY_LOWER.get(key.lower())
        if canonical:
            self._select_value = canonical
            self._custom_key = ""
        elif key:
            self._select_value = CUSTOM
            self._custom_key = key
        else:
            self._select_value = Select.NULL
            self._custom_key = ""
        self._value = value

    def compose(self) -> ComposeResult:
        options = [(d, d) for d in COMMON_DIRECTIVES] + [("Custom...", CUSTOM)]
        yield Select(
            options,
            value=self._select_value,
            prompt="Directive",
            classes="kv-key-select",
        )
        yield Input(
            value=self._custom_key,
            placeholder="Custom directive name",
            classes="kv-key-custom",
        )
        yield Input(value=self._value, placeholder="Value", classes="kv-value")
        yield Button("✕", classes="kv-remove", variant="error")

    def on_mount(self) -> None:
        self._sync_custom_visibility()

    def on_select_changed(self, event: Select.Changed) -> None:
        if "kv-key-select" in event.select.classes:
            event.stop()
            self._sync_custom_visibility()

    def _sync_custom_visibility(self) -> None:
        select = self.query_one(".kv-key-select", Select)
        custom_input = self.query_one(".kv-key-custom", Input)
        custom_input.display = select.value == CUSTOM

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if "kv-remove" in event.button.classes:
            event.stop()
            self.remove()

    @property
    def key(self) -> str:
        select = self.query_one(".kv-key-select", Select)
        if select.value == CUSTOM:
            return self.query_one(".kv-key-custom", Input).value.strip()
        if select.value is Select.NULL:
            return ""
        return str(select.value)

    @property
    def value_text(self) -> str:
        return self.query_one(".kv-value", Input).value.strip()


class HostEditScreen(ModalScreen[tuple[str, dict[str, str]] | None]):
    def __init__(
        self,
        alias: str | None = None,
        existing: dict | None = None,
        initial_alias: str | None = None,
        title_override: str | None = None,
        taken_aliases: frozenset[str] = frozenset(),
    ) -> None:
        super().__init__()
        self.editing_alias = alias
        self.existing = existing or {}
        self.initial_alias = initial_alias if initial_alias is not None else (alias or "")
        self.title_override = title_override
        self.taken_aliases = taken_aliases

    def compose(self) -> ComposeResult:
        title = self.title_override or (
            f"Edit host: {self.editing_alias}" if self.editing_alias else "Add host"
        )
        with Vertical(id="edit-dialog"):
            yield Static(title, id="edit-title")
            yield Label("Host (alias)")
            yield Input(value=self.initial_alias, id="field-alias")
            yield Static("", id="alias-error")
            for key, label in CORE_LABELS.items():
                yield Label(label)
                yield Input(
                    value=self._flatten(self.existing.get(key, "")), id=f"field-{key}"
                )
            yield Label("Other parameters")
            with VerticalScroll(id="kv-rows"):
                for key, value in self.existing.items():
                    if key in CORE_LABELS:
                        continue
                    yield KeyValueRow(key, self._flatten(value))
            with Horizontal(id="edit-buttons"):
                yield Button("+ Add parameter", id="add-row")
                yield Button("Save", id="save", variant="success")
                yield Button("Cancel", id="cancel", variant="error")

    @staticmethod
    def _flatten(value: object) -> str:
        if isinstance(value, list):
            return ", ".join(str(v) for v in value)
        return str(value) if value else ""

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "add-row":
            self.query_one("#kv-rows", VerticalScroll).mount(KeyValueRow())
        elif event.button.id == "save":
            self._save()
        elif event.button.id == "cancel":
            self.dismiss(None)

    def _show_alias_error(self, message: str) -> None:
        self.query_one("#alias-error", Static).update(message)

    def _save(self) -> None:
        alias = self.query_one("#field-alias", Input).value.strip()
        if not alias:
            self._show_alias_error("Alias cannot be empty.")
            return
        # Editing a host and keeping its own alias is fine; anything else
        # colliding with an existing host (including two clones both left
        # on the same suggested "-copy" name) is not.
        if alias != self.editing_alias and alias in self.taken_aliases:
            self._show_alias_error(f"'{alias}' already exists - pick another name.")
            return
        params: dict[str, str] = {}
        for key in CORE_LABELS:
            value = self.query_one(f"#field-{key}", Input).value.strip()
            if value:
                params[key] = value
        for row in self.query(KeyValueRow):
            key = row.key.lower()
            value = row.value_text
            if key and value:
                params[key] = value
        self.dismiss((alias, params))
