"""Thin wrapper around sshconf for reading/editing ~/.ssh/config."""

from __future__ import annotations

import re
import shutil
import time
from pathlib import Path

from sshconf import SshConfigFile, empty_ssh_config_file

CONFIG_PATH = Path.home() / ".ssh" / "config"
BACKUP_DIR = Path.home() / ".config" / "sshtui" / "backups"

# Fields shown as dedicated inputs in the edit form; everything else in a
# host's params is treated as a generic key/value row.
CORE_FIELDS = ("hostname", "user", "port")

_backed_up = False

_EQUALS_DIRECTIVE_RE = re.compile(r"^([A-Za-z][A-Za-z0-9]*)=(.*)$")


def _normalize_equals_syntax(lines: list[str]) -> list[str]:
    """ssh_config allows both `Key value` and `Key=value`, but the sshconf
    parser only understands the space-separated form: a `Key=value` line
    gets silently detached from its host block (no host/key/value on the
    ConfigLine), so it never shows up in host() and would be orphaned if
    the host were removed. Rewrite to `Key value` on load so every
    directive is actually visible and editable."""
    normalized = []
    for line in lines:
        indent = line[: len(line) - len(line.lstrip())]
        rest = line[len(indent):]
        code, sep, comment = rest.partition("#")
        code_stripped = code.strip()
        if code_stripped and " " not in code_stripped and "\t" not in code_stripped:
            match = _EQUALS_DIRECTIVE_RE.match(code_stripped)
            if match:
                key, value = match.groups()
                new_rest = f"{key} {value}" + (f"#{comment}" if sep else "")
                normalized.append(f"{indent}{new_rest}")
                continue
        normalized.append(line)
    return normalized


def load() -> SshConfigFile:
    if not CONFIG_PATH.exists():
        return empty_ssh_config_file()
    raw_lines = CONFIG_PATH.read_text().splitlines()
    return SshConfigFile(_normalize_equals_syntax(raw_lines))


def list_aliases(conf: SshConfigFile) -> list[str]:
    return [h for h in conf.hosts() if h != "*"]


def host_params(conf: SshConfigFile, alias: str) -> dict[str, str]:
    return conf.host(alias)


# ssh_config directives can legitimately repeat (multiple IdentityFile
# entries being the common case), and sshconf.host() returns a list for
# any key that appeared more than once. These helpers normalize that for
# the different things callers need it for: a single display string, a
# comparable form for change detection, or one representative value.


def flatten_value(value: object) -> str:
    """Join a possibly-multi-value directive into one display string."""
    if isinstance(value, list):
        return ", ".join(str(v) for v in value)
    return str(value) if value else ""


def normalize_value(value: object) -> tuple[str, ...]:
    """Normalize into a comparable tuple, so change-detection works the
    same whether a directive has one value or several."""
    if isinstance(value, list):
        return tuple(str(v) for v in value)
    return (str(value),) if value else ()


def primary_value(value: object) -> str:
    """Pick one representative value from a possibly-multi-value
    directive, for uses (like a reachability check) where only one makes
    sense - matches ssh's own first-occurrence-wins precedence."""
    if isinstance(value, list):
        return str(value[0]) if value else ""
    return str(value) if value else ""


def alias_segments(alias: str) -> list[str]:
    """Split an alias into hierarchical group segments on "--", with the
    last segment being the leaf's own display name. "srv--nas--trunas..."
    -> ["srv", "nas", "trunas..."] (nests two levels deep); a bare alias
    with no "--" at all falls into a single "other" group."""
    if "--" not in alias:
        return ["other", alias]
    return alias.split("--")


def save(conf: SshConfigFile) -> None:
    """Backup ~/.ssh/config once per process, then write changes."""
    global _backed_up
    if not _backed_up and CONFIG_PATH.exists():
        BACKUP_DIR.mkdir(parents=True, exist_ok=True)
        stamp = time.strftime("%Y%m%dT%H%M%S")
        shutil.copy2(CONFIG_PATH, BACKUP_DIR / f"config-{stamp}")
        _backed_up = True
    CONFIG_PATH.parent.mkdir(parents=True, exist_ok=True)
    conf.write(str(CONFIG_PATH))


def add_host(conf: SshConfigFile, alias: str, params: dict[str, str]) -> None:
    conf.add(alias, **params)


def update_host(
    conf: SshConfigFile,
    alias: str,
    params: dict[str, str],
    removed_keys: list[str] | None = None,
) -> None:
    if params:
        # sshconf.set() reassigns multi-value lists via list.pop(), which
        # consumes from the end - so a list handed to it comes out
        # reversed on disk. Order matters for directives like IdentityFile
        # (ssh tries them in the order listed), so pre-reverse here to
        # cancel that out. Only set() does this; add() (new hosts) appends
        # in the given order correctly and needs no compensation.
        params = {
            k: list(reversed(v)) if isinstance(v, list) else v
            for k, v in params.items()
        }
        conf.set(alias, **params)
    if removed_keys:
        conf.unset(alias, *removed_keys)


def remove_host(conf: SshConfigFile, alias: str) -> None:
    conf.remove(alias)
