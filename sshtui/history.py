"""Tracks recently/frequently used hosts for sshtui's default sort order."""

from __future__ import annotations

import json
import time
from pathlib import Path

HISTORY_PATH = Path.home() / ".config" / "sshtui" / "history.json"


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


def sort_aliases(aliases: list[str]) -> list[str]:
    """Most-recently-used first (ties broken by count); never-used hosts last, alphabetically."""
    data = _load()
    used = [a for a in aliases if a in data]
    unused = sorted(a for a in aliases if a not in data)
    used.sort(key=lambda a: (data[a]["last_used"], data[a]["count"]), reverse=True)
    return used + unused
