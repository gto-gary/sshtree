"""Background TCP reachability checks for hosts, used to drive the status dot."""

from __future__ import annotations

import asyncio
import socket
from collections.abc import AsyncIterator

TIMEOUT = 1.5
DEFAULT_PORT = 22


def resolve_port(value: str) -> int:
    """Resolve an ssh_config Port value to an integer. ssh_config allows
    named ports (e.g. "ssh", "http"), resolved the same way ssh itself
    does - via the system's service database (/etc/services) - not just
    plain numbers. Falls back to DEFAULT_PORT if the value is empty or
    doesn't resolve either way, rather than raising."""
    if not value:
        return DEFAULT_PORT
    try:
        return int(value)
    except ValueError:
        pass
    try:
        return socket.getservbyname(value, "tcp")
    except OSError:
        return DEFAULT_PORT


async def check_host(host: str, port: int = DEFAULT_PORT) -> bool:
    try:
        _, writer = await asyncio.wait_for(
            asyncio.open_connection(host, port), timeout=TIMEOUT
        )
    except (OSError, asyncio.TimeoutError):
        return False
    writer.close()
    try:
        await writer.wait_closed()
    except OSError:
        pass
    return True


async def check_all(
    targets: dict[str, tuple[str, int]],
) -> AsyncIterator[tuple[str, bool]]:
    """targets: {alias: (hostname, port)}. Yields (alias, reachable) as each resolves."""

    async def _one(alias: str, host: str, port: int) -> tuple[str, bool]:
        return alias, await check_host(host, port)

    tasks = [
        asyncio.create_task(_one(alias, host, port))
        for alias, (host, port) in targets.items()
    ]
    for coro in asyncio.as_completed(tasks):
        yield await coro
