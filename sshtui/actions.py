"""What HostListScreen exits with: connect via ssh, or copy a file via scp."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ConnectAction:
    alias: str


@dataclass(frozen=True)
class ScpAction:
    alias: str
    local_path: str
    remote_path: str
    upload: bool  # True: local -> remote. False: remote -> local.
