"""历史记录：JSON Lines，追加写入（不再每次读改写整个文件，崩溃/断电最多丢最后一行）。

旧版 history.json 不会被读取或改动；v2 记录写入 history.jsonl。
"""
from __future__ import annotations

import json
import os
import sys
from pathlib import Path

DEFAULT_PATH = "history.jsonl"


def resolve_path(path: str | None) -> Path:
    return Path(path or os.environ.get("DIVINATION_HISTORY") or DEFAULT_PATH)


def append(path: Path, record: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as f:
        f.write(json.dumps(record, ensure_ascii=False) + "\n")
        f.flush()
        os.fsync(f.fileno())


def load(path: Path) -> list[dict]:
    if not path.exists():
        return []
    records = []
    for no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        try:
            records.append(json.loads(line))
        except json.JSONDecodeError:
            print(f"警告：{path} 第 {no} 行不是合法 JSON，已跳过", file=sys.stderr)
    return records
