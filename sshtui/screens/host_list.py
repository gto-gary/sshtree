"""Main screen: browse hosts grouped by prefix, connect, add/edit/delete."""

from __future__ import annotations

from importlib.metadata import PackageNotFoundError, version as _pkg_version

from rich.markup import escape as markup_escape
from textual import events, work
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal
from textual.screen import Screen
from textual.widgets import Footer, Header, Input, Static, Tree
from textual.widgets.tree import TreeNode

from .. import config, history, reachability
from ..actions import ConnectAction, ScpAction, SftpAction
from .confirm import ConfirmScreen
from .host_edit import HostEditScreen
from .scp import ScpScreen

STATUS_ICON = {"unknown": "○", "up": "●", "down": "●"}
STATUS_COLOR = {"unknown": "grey50", "up": "green", "down": "red"}


def _version_str() -> str:
    try:
        return f"v{_pkg_version('sshtui')}"
    except PackageNotFoundError:
        return "(dev)"


BANNER_TEXT = f"\U0001f511 sshtui [dim]{_version_str()}[/]"


class _GroupNode:
    """One level of the alias-prefix hierarchy: subgroups plus leaf hosts
    that terminate at this level. Built fresh from config.alias_segments()
    on every render, so it naturally supports any nesting depth - e.g.
    "srv--nas--trunas..." nests two levels under "srv" > "nas"."""

    __slots__ = ("children", "leaves")

    def __init__(self) -> None:
        self.children: dict[str, "_GroupNode"] = {}
        self.leaves: dict[str, str] = {}  # display_name -> full alias


def _build_group_tree(aliases: list[str]) -> _GroupNode:
    root = _GroupNode()
    for alias in aliases:
        segments = config.alias_segments(alias)
        node = root
        for part in segments[:-1]:
            node = node.children.setdefault(part, _GroupNode())
        node.leaves[segments[-1]] = alias
    return root


class HostTree(Tree):
    """Tree widget that syncs its horizontal scroll position with #column-header."""

    def watch_scroll_x(self, old_value: float, new_value: float) -> None:
        super().watch_scroll_x(old_value, new_value)
        try:
            header = self.screen.query_one("#column-header", Static)
            header.styles.offset = (-int(new_value), 0)
        except Exception:
            pass


class HostListScreen(Screen):
    BINDINGS = [
        Binding("a", "add_host", "Add"),
        Binding("e", "edit_host", "Edit"),
        Binding("c", "clone_host", "Clone"),
        Binding("f", "sftp_host", "SFTP"),
        Binding("s", "scp_host", "SCP copy"),
        Binding("d", "delete_host", "Delete"),
        Binding("r", "refresh_reachability", "Refresh"),
        Binding("slash", "focus_search", "Search", show=True),
        Binding("escape", "clear_search", "Clear search"),
        Binding("q", "quit", "Quit"),
    ]

    def compose(self) -> ComposeResult:
        yield Header()
        with Horizontal(id="banner-row"):
            yield Static(BANNER_TEXT, id="banner")
            yield Static("", id="banner-stats")
        yield Input(
            placeholder="Search (alias, hostname, user, port; or extra:yes/no, status:down)...",
            id="search-input",
        )
        yield Static("", id="column-header")
        yield HostTree("Hosts", id="host-tree")
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
        self._col_widths = self._compute_col_widths(aliases)
        self._update_column_header()
        self._alias_nodes = {}
        group_tree = _build_group_tree(aliases)
        self._render_group(group_tree, tree.root)
        self._update_banner_stats()

    def _render_group(self, group: "_GroupNode", tree_node: TreeNode) -> None:
        for name in sorted(group.children):
            child_node = tree_node.add(name, expand=True)
            self._render_group(group.children[name], child_node)
        for display_name in sorted(group.leaves):
            alias = group.leaves[display_name]
            params = config.host_params(self.conf, alias)
            label = self._row_label(
                alias, display_name, params, self._status.get(alias, "unknown")
            )
            node = tree_node.add_leaf(label, data=alias)
            self._alias_nodes[alias] = node

    def _matches_filter(self, alias: str) -> bool:
        text = self._filter_text
        params = config.host_params(self.conf, alias)
        if text.startswith("extra:"):
            has_extra = any(k not in config.CORE_FIELDS for k in params)
            want = text.split(":", 1)[1].strip()
            if want in ("yes", "y", "true"):
                return has_extra
            if want in ("no", "n", "false"):
                return not has_extra
            return False
        if text.startswith("status:"):
            status = self._status.get(alias, "unknown")
            want = text.split(":", 1)[1].strip()
            if want in ("down", "unreachable"):
                return status == "down"
            if want in ("up", "reachable"):
                return status == "up"
            if want in ("unknown", "checking"):
                return status == "unknown"
            return False
        port = config.flatten_value(params.get("port", "22"))
        haystack = " ".join(
            [
                alias,
                config.flatten_value(params.get("hostname", "")),
                config.flatten_value(params.get("user", "")),
                port,
            ]
        ).lower()
        return text in haystack

    def _compute_col_widths(self, aliases: list[str]) -> dict[str, int]:
        widths = {"alias": len("Alias"), "hostname": len("Hostname"), "user": len("User")}
        for alias in aliases:
            params = config.host_params(self.conf, alias)
            display_name = config.alias_segments(alias)[-1]
            hostname = config.flatten_value(params.get("hostname", ""))
            user = config.flatten_value(params.get("user", ""))
            widths["alias"] = max(widths["alias"], len(display_name))
            widths["hostname"] = max(widths["hostname"], len(hostname))
            widths["user"] = max(widths["user"], len(user))
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
        col_header = self.query_one("#column-header", Static)
        col_header.update(header)
        tree = self.query_one("#host-tree", Tree)
        col_header.styles.offset = (-int(tree.scroll_x), 0)

    def _update_banner_stats(self) -> None:
        total = len(self._alias_nodes)
        up = sum(1 for a in self._alias_nodes if self._status.get(a) == "up")
        down = sum(1 for a in self._alias_nodes if self._status.get(a) == "down")
        checking = total - up - down
        parts = [f"{total} host{'s' if total != 1 else ''}"]
        if total:
            parts.append(f"[{STATUS_COLOR['up']}]{up}[/] up")
            parts.append(f"[{STATUS_COLOR['down']}]{down}[/] down")
            if checking:
                parts.append(f"[{STATUS_COLOR['unknown']}]{checking}[/] checking")
        self.query_one("#banner-stats", Static).update(" · ".join(parts))

    def _row_label(
        self, alias: str, display_name: str, params: dict, status: str
    ) -> str:
        icon = STATUS_ICON[status]
        color = STATUS_COLOR[status]
        hostname = config.flatten_value(params.get("hostname", ""))
        user = config.flatten_value(params.get("user", ""))
        port = config.flatten_value(params.get("port", "22"))
        has_extra = any(k not in config.CORE_FIELDS for k in params)
        extra = "Yes" if has_extra else "No"
        w = self._col_widths
        # Values ultimately come from ~/.ssh/config, not just what was
        # typed through this app's own forms - escape them before
        # interpolating into Rich markup so a hostname/user containing
        # literal "[...]" text (accidentally or from an untrusted/shared
        # config) can't be interpreted as styling.
        alias_col = markup_escape(display_name.ljust(w["alias"]))
        hostname_col = markup_escape(hostname.ljust(w["hostname"]))
        port_col = markup_escape(port.rjust(5))
        user_col = markup_escape(user.ljust(w["user"]))
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
        # Exit with the action as the result; app.py execs ssh only after
        # Textual has fully torn down and restored the terminal, otherwise
        # ssh's password prompt renders into a still-raw/alt-screen terminal
        # and appears to hang.
        self.app.exit(ConnectAction(alias))

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
            if config.normalize_value(existing.get(key, ""))
            != config.normalize_value(value)
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
    async def action_scp_host(self) -> None:
        alias = self.selected_alias()
        if alias is None:
            return
        result = await self.app.push_screen_wait(ScpScreen(alias))
        if result is None:
            return
        history.record_use(alias)
        # Same reasoning as connect: exit cleanly first, exec scp only
        # after Textual has released the terminal, so its progress bar
        # and any password/passphrase prompt behave normally.
        self.app.exit(result)

    def action_sftp_host(self) -> None:
        alias = self.selected_alias()
        if alias is None:
            return
        history.record_use(alias)
        # Same reasoning as connect: exit cleanly first, exec sftp only
        # after Textual has released the terminal.
        self.app.exit(SftpAction(alias))

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
            # A directive repeated in the host block (e.g. Hostname or
            # Port accidentally listed twice) comes back as a list from
            # sshconf - take the first value, matching ssh's own
            # first-occurrence-wins precedence, rather than crashing.
            host = config.primary_value(params.get("hostname", alias)) or alias
            port_str = config.primary_value(params.get("port", ""))
            # Port can be a named service (e.g. "ssh", "http"), resolved
            # the same way ssh itself resolves it - not just a number.
            port = reachability.resolve_port(port_str)
            targets[alias] = (host, port)
            self._status[alias] = "unknown"
            node = self._alias_nodes[alias]
            display_name = config.alias_segments(alias)[-1]
            node.set_label(self._row_label(alias, display_name, params, "unknown"))
        self._update_banner_stats()
        async for alias, ok in reachability.check_all(targets):
            self._status[alias] = "up" if ok else "down"
            node = self._alias_nodes.get(alias)
            if node is not None:
                params = config.host_params(self.conf, alias)
                display_name = config.alias_segments(alias)[-1]
                node.set_label(
                    self._row_label(alias, display_name, params, self._status[alias])
                )
            self._update_banner_stats()
