"""语料层：加载 data/iching.json 并提供查询。"""
from __future__ import annotations

import json
from functools import lru_cache
from pathlib import Path

from .hexagram import check_code

DATA_PATH = Path(__file__).resolve().parent / "data" / "iching.json"


class IChing:
    def __init__(self, path: Path | str = DATA_PATH) -> None:
        raw = json.loads(Path(path).read_text(encoding="utf-8"))
        self.meta: dict = raw["meta"]
        self.hexagrams: dict[str, dict] = raw["hexagrams"]
        self._by_id = {h["id"]: code for code, h in self.hexagrams.items()}
        self._by_name = {}
        for code, h in self.hexagrams.items():
            for key in (h["name"], h["full_name"], h["full_name"].removesuffix("卦")):
                self._by_name[key] = code

    def get(self, code: str) -> dict:
        return self.hexagrams[check_code(code)]

    def code_of_id(self, hid: int) -> str:
        return self._by_id[hid]

    def find(self, key: str) -> str | None:
        """按 卦码 / 序号 / 卦名 / 全名 查找，返回卦码；找不到返回 None。"""
        key = key.strip()
        if key in self.hexagrams:
            return key
        if key.isdigit() and int(key) in self._by_id:
            return self._by_id[int(key)]
        return self._by_name.get(key)

    def name(self, code: str) -> str:
        return self.get(code)["name"]


@lru_cache(maxsize=1)
def load() -> IChing:
    return IChing()
