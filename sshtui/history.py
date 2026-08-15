"""Tracks recently/frequently used hosts (usage count, last-used time).

Not currently used to drive display order (hosts sort alphabetically),
but kept as a record in case that's wanted again later."""

from __future__ import annotations

import os
import json
import time
from pathlib import Path

CONFIG_HOME = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config"))
HISTORY_PATH = CONFIG_HOME / "sshtui" / "history.json"


def _load() -> dict[str, dict]:
    if not HISTORY_PATH.exists():
        return {}
    try:
        return json.loads(HISTORY_PATH.read_text())
    except (json.JSONDecodeError, OSError):
        return {}


def record_use(alias: str) -> None:
    data = _load()
    entry = data.setdefault(alias, {"count": 0, "last_used": ""})
    entry["count"] += 1
    entry["last_used"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    HISTORY_PATH.parent.mkdir(parents=True, exist_ok=True)
    HISTORY_PATH.write_text(json.dumps(data, indent=2))
