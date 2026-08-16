"""What HostListScreen exits with: connect via ssh, drop into an sftp
prompt, or copy a file via scp.

Each action knows how to render itself as newline-separated lines for
--print-only mode. A shell wrapper (see shell/) captures that output via
$(...) and runs the real ssh/sftp/scp command itself, *after* the capture
has completed - so the real command gets a normal terminal, not one whose
stdout is being captured. One line per field keeps the shell-side parsing
trivial (no shell-quoting needed to survive spaces in paths)."""

from __future__ import annotations

import time
from dataclasses import dataclass
from pathlib import Path

SESSION_LOG_DIR = Path.home() / "Documents" / "sshtui"


def session_log_path(alias: str) -> Path:
    """Computed once per recorded connection (not re-derived per code path,
    which would risk two different timestamps for the same session)."""
    stamp = time.strftime("%Y%m%dT%H%M%S")
    return SESSION_LOG_DIR / f"{alias}-{stamp}.log"


@dataclass(frozen=True)
class ConnectAction:
    alias: str
    record_to: Path | None = None  # log path if this session should be recorded

    def print_only_lines(self) -> list[str]:
        if self.record_to is not None:
            return ["record", self.alias, str(self.record_to)]
        return ["connect", self.alias]


@dataclass(frozen=True)
class SftpAction:
    alias: str

    def print_only_lines(self) -> list[str]:
        return ["sftp", self.alias]


@dataclass(frozen=True)
class ScpAction:
    alias: str
    local_path: str
    remote_path: str
    upload: bool  # True: local -> remote. False: remote -> local.

    def print_only_lines(self) -> list[str]:
        direction = "upload" if self.upload else "download"
        # "~" isn't expanded by the shell when it comes from a variable
        # substitution (only a literal ~ on the command line triggers
        # that) - expand it here so the wrapper doesn't need to.
        local = str(Path(self.local_path).expanduser())
        return ["scp", direction, self.alias, local, self.remote_path]
