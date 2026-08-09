"""Main screen: browse hosts grouped by prefix, connect, add/edit/delete."""

from __future__ import annotations

from importlib.metadata import PackageNotFoundError, version as _pkg_version

from textual import events, work
from textual.app import ComposeResult
from textual.binding import Binding
from textual.screen import Screen
from textual.widgets import Footer, Header, Input, Static, Tree
from textual.widgets.tree import TreeNode

from .. import config, history, reachability
from .confirm import ConfirmScreen
from .host_edit import HostEditScreen

STATUS_ICON = {"unknown": "○", "up": "●", "down": "●"}
STATUS_COLOR = {"unknown": "grey50", "up": "green", "down": "red"}

BANNER_ART = (
    r"         _     _         _ " "\n"
    r" ___ ___| |__ | |_ _   _(_)" "\n"
    r"/ __/ __| '_ \| __| | | | |" "\n"
    r"\__ \__ \ | | | |_| |_| | |" "\n"
    r"|___/___/_| |_|\__|\__,_|_|"
)
_BANNER_WIDTH = max(len(line) for line in BANNER_ART.splitlines())


def _version_line() -> str:
    try:
        v = f"v{_pkg_version('sshtui')}"
    except PackageNotFoundError:
        v = "(dev)"
    return v.center(_BANNER_WIDTH)


BANNER_TEXT = f"{BANNER_ART}\n{_version_line()}"


class HostListScreen(Screen):
    BINDINGS = [
        Binding("a", "add_host", "Add"),
        Binding("e", "edit_host", "Edit"),
        Binding("c", "clone_host", "Clone"),
        Binding("d", "delete_host", "Delete"),
        Binding("r", "refresh_reachability", "Refresh"),
        Binding("slash", "focus_search", "Search", show=True),
        Binding("escape", "clear_search", "Clear search"),
        Binding("q", "quit", "Quit"),
    ]

    def compose(self) -> ComposeResult:
        yield Header()
        yield Static(BANNER_TEXT, id="banner")
        yield Input(placeholder="Search hosts (alias, hostname, user)...", id="search-input")
        yield Static("", id="column-header")
        yield Tree("Hosts", id="host-tree")
        yield Footer()

    def on_mount(self) -> None:
        self._status: dict[str, str] = {}
        self._alias_nodes: dict[str, TreeNode] = {}
        self._filter_text: str = ""
        self.conf = config.load()
        self.render_tree()
        self.check_reachability()
        # Input is first in compose() order, so Textual would otherwise give
        # it default focus on mount - meaning every keybinding (a/e/d/r/q)
        # would just type into the search box until the user clicked the
        # tree. Explicitly focus the tree instead.
        self.query_one("#host-tree", Tree).focus()

    def rebuild_tree(self) -> None:
        """Reload ~/.ssh/config from disk and re-render (data may have changed)."""
        self.conf = config.load()
        self.render_tree()

    def render_tree(self) -> None:
        """Re-render the tree from the already-loaded config, applying the
        current search filter. Does not touch disk - safe to call on every
        keystroke while searching, even with hundreds of hosts."""
        tree = self.query_one("#host-tree", Tree)
        tree.clear()
        tree.root.expand()
        aliases = config.list_aliases(self.conf)
        if self._filter_text:
            aliases = [a for a in aliases if self._matches_filter(a)]
        ordered = history.sort_aliases(aliases)
        order_index = {alias: i for i, alias in enumerate(ordered)}
        groups = config.group_by_prefix(aliases)
        self._col_widths = self._compute_col_widths(aliases)
        self._update_column_header()
        self._alias_nodes = {}
        for group_name in sorted(groups):
            group_aliases = sorted(groups[group_name], key=lambda a: order_index[a])
            group_node = tree.root.add(group_name, expand=True)
            for alias in group_aliases:
                params = config.host_params(self.conf, alias)
                label = self._row_label(alias, params, self._status.get(alias, "unknown"))
                node = group_node.add_leaf(label, data=alias)
                self._alias_nodes[alias] = node

    def _matches_filter(self, alias: str) -> bool:
        params = config.host_params(self.conf, alias)
        haystack = " ".join(
            [alias, params.get("hostname", ""), params.get("user", "")]
        ).lower()
        return self._filter_text in haystack

    def _compute_col_widths(self, aliases: list[str]) -> dict[str, int]:
        widths = {"alias": len("Alias"), "hostname": len("Hostname"), "user": len("User")}
        for alias in aliases:
            params = config.host_params(self.conf, alias)
            display_alias = alias.split("--", 1)[1] if "--" in alias else alias
            widths["alias"] = max(widths["alias"], len(display_alias))
            widths["hostname"] = max(widths["hostname"], len(params.get("hostname", "")))
            widths["user"] = max(widths["user"], len(params.get("user", "")))
        return widths

    # Tree indents leaf rows by 2 guide levels (root -> group -> leaf) at
    # guide_depth=4 each, plus its own node-icon slot - empirically the
    # leaf label starts 10 columns in. Used only to roughly line up the
    # header; row-to-row alignment (the part that actually matters for
    # readability) comes from _col_widths and doesn't depend on this.
    _TREE_LEAF_INDENT = 10

    def _update_column_header(self) -> None:
        w = self._col_widths
        header = (
            " " * self._TREE_LEAF_INDENT
            + "Alias".ljust(w["alias"])
            + "  "
            + "Hostname".ljust(w["hostname"])
            + "  "
            + "Port".rjust(5)
            + "  "
            + "User".ljust(w["user"])
            + "  Extra"
        )
        self.query_one("#column-header", Static).update(header)

    def _row_label(self, alias: str, params: dict, status: str) -> str:
        icon = STATUS_ICON[status]
        color = STATUS_COLOR[status]
        hostname = params.get("hostname", "")
        user = params.get("user", "")
        port = params.get("port", "22")
        display_alias = alias.split("--", 1)[1] if "--" in alias else alias
        has_extra = any(k not in config.CORE_FIELDS for k in params)
        extra = "Yes" if has_extra else "No"
        w = self._col_widths
        alias_col = display_alias.ljust(w["alias"])
        hostname_col = hostname.ljust(w["hostname"])
        port_col = str(port).rjust(5)
        user_col = user.ljust(w["user"])
        return (
            f"[{color}]{icon}[/] {alias_col}  [dim]{hostname_col}[/]  "
            f"{port_col}  {user_col}  {extra}"
        )

    def selected_alias(self) -> str | None:
        tree = self.query_one("#host-tree", Tree)
        node = tree.cursor_node
        if node is None or node.data is None:
            return None
        return str(node.data)

    def on_key(self, event: events.Key) -> None:
        # Intercept "/" as early as possible, before it can also reach the
        # newly-focused Input's own printable-character handling: the
        # Binding/action path alone isn't enough to suppress that, and the
        # same "/" keypress ends up typed into the search box otherwise.
        search = self.query_one("#search-input", Input)
        if event.key == "slash" and not search.has_focus:
            event.stop()
            event.prevent_default()
            search.focus()

    def action_focus_search(self) -> None:
        self.query_one("#search-input", Input).focus()

    def action_clear_search(self) -> None:
        search = self.query_one("#search-input", Input)
        if search.value:
            search.value = ""  # triggers on_input_changed, which re-renders
        self.query_one("#host-tree", Tree).focus()

    def on_input_changed(self, event: Input.Changed) -> None:
        if event.input.id == "search-input":
            self._filter_text = event.value.strip().lower()
            self.render_tree()

    def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.input.id == "search-input":
            self.query_one("#host-tree", Tree).focus()

    def on_tree_node_selected(self, event: Tree.NodeSelected) -> None:
        # Tree's own "enter" binding fires this; group nodes have no data.
        alias = event.node.data
        if alias is None:
            return
        history.record_use(alias)
        # Exit with the alias as the result; app.py execs ssh only after
        # Textual has fully torn down and restored the terminal, otherwise
        # ssh's password prompt renders into a still-raw/alt-screen terminal
        # and appears to hang.
        self.app.exit(alias)

    def _taken_aliases(self) -> frozenset[str]:
        return frozenset(config.list_aliases(self.conf))

    def _unique_clone_alias(self, source: str) -> str:
        taken = self._taken_aliases()
        candidate = f"{source}-copy"
        n = 2
        while candidate in taken:
            candidate = f"{source}-copy{n}"
            n += 1
        return candidate

    @work
    async def action_add_host(self) -> None:
        result = await self.app.push_screen_wait(
            HostEditScreen(taken_aliases=self._taken_aliases())
        )
        if result is None:
            return
        alias, params = result
        config.add_host(self.conf, alias, params)
        config.save(self.conf)
        self.rebuild_tree()
        self.check_reachability()

    @work
    async def action_edit_host(self) -> None:
        alias = self.selected_alias()
        if alias is None:
            return
        existing = config.host_params(self.conf, alias)
        result = await self.app.push_screen_wait(
            HostEditScreen(
                alias=alias, existing=existing, taken_aliases=self._taken_aliases()
            )
        )
        if result is None:
            return
        new_alias, params = result
        if new_alias != alias:
            self.conf.rename(alias, new_alias)
            alias = new_alias
        removed_keys = [key for key in existing if key not in params]
        # Only pass values that actually changed. sshconf's set() rewrites
        # a line's formatting whenever it's called for that key, even if
        # the value is identical - so blindly resubmitting every field
        # (the form always includes all core fields) would silently
        # reformat lines the user never touched.
        changed_params = {
            key: value
            for key, value in params.items()
            if HostEditScreen._flatten(existing.get(key, "")) != value
        }
        config.update_host(self.conf, alias, changed_params, removed_keys=removed_keys)
        config.save(self.conf)
        self.rebuild_tree()
        self.check_reachability()

    @work
    async def action_clone_host(self) -> None:
        source = self.selected_alias()
        if source is None:
            return
        existing = config.host_params(self.conf, source)
        result = await self.app.push_screen_wait(
            HostEditScreen(
                existing=existing,
                initial_alias=self._unique_clone_alias(source),
                title_override=f"Clone host: {source}",
                taken_aliases=self._taken_aliases(),
            )
        )
        if result is None:
            return
        new_alias, params = result
        config.add_host(self.conf, new_alias, params)
        config.save(self.conf)
        self.rebuild_tree()
        self.check_reachability()

    @work
    async def action_delete_host(self) -> None:
        alias = self.selected_alias()
        if alias is None:
            return
        confirmed = await self.app.push_screen_wait(
            ConfirmScreen(f"Delete host '{alias}'?")
        )
        if not confirmed:
            return
        config.remove_host(self.conf, alias)
        config.save(self.conf)
        self.rebuild_tree()

    def action_quit(self) -> None:
        self.app.exit()

    def action_refresh_reachability(self) -> None:
        self.check_reachability()

    @work(exclusive=True)
    async def check_reachability(self) -> None:
        targets: dict[str, tuple[str, int]] = {}
        for alias in self._alias_nodes:
            params = config.host_params(self.conf, alias)
            host = params.get("hostname", alias)
            port = int(params.get("port", reachability.DEFAULT_PORT))
            targets[alias] = (host, port)
            self._status[alias] = "unknown"
            node = self._alias_nodes[alias]
            node.set_label(self._row_label(alias, params, "unknown"))
        async for alias, ok in reachability.check_all(targets):
            self._status[alias] = "up" if ok else "down"
            node = self._alias_nodes.get(alias)
            if node is not None:
                params = config.host_params(self.conf, alias)
                node.set_label(self._row_label(alias, params, self._status[alias]))
