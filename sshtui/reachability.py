"""Background TCP reachability checks for hosts, used to drive the status dot."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator

TIMEOUT = 1.5
DEFAULT_PORT = 22


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
