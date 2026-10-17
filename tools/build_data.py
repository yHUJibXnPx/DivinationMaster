#!/usr/bin/env python3
"""由三份来源重建 divination/data/iching.json（可重复执行，只用标准库）。

来源（均在 tools/sources/）：
  open_iching.json        卦名、卦象数组、卦辞、384 条爻辞 + 乾「用九」、坤「用六」
  open_iching_xiang.json  《象传》（大象 + 每爻小象 + 用九/用六小象）
  legacy_hexagram.json    旧版 hexagram.json 里的卦辞白话解释（interpretation）

修正旧数据的两处问题：
  1. 旧数据把「剥」误标为 000011（那是「观」的卦码），剥应为 000001；
     本脚本按"第 N 卦"的序号对齐，而不是按旧卦码对齐。
  2. 旧数据缺第 20 卦「观」，这里补上（白话为新写）。

用法（在项目根目录）：python tools/build_data.py
"""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "tools" / "sources"
OUT = ROOT / "divination" / "data" / "iching.json"
GO_OUT = ROOT / "go" / "internal" / "corpus" / "iching.json"  # Go 用 go:embed 内嵌，必须在包目录内

POSITIONS = ("初", "二", "三", "四", "五", "上")

# 观卦的白话解释：旧数据缺失，此处为新写。
GUAN_INTERPRETATION = (
    "观卦取象于风行地上，象征观察与示范。卦辞说祭祀时已洗手净身、尚未献上祭品，"
    "心中却已诚敬庄严——真正有感召力的是内在的诚信与庄重，而不是表面的排场。"
    "得此卦宜多观察、少妄动，先看清局势与自身，再决定如何行事；"
    "同时也提醒：自己的言行本身，就是别人观望的对象。"
)
GUAN_META = {"pinyin": "guān", "full_name": "风地观卦"}


def fix_seams(text: str, name: str) -> str:
    """旧白话由"摘要 + 详解"两段拼成，其中不少在接缝处漏了句号，如
    「得此卦者不可独立创业坤象征大地」。特征很明确：「得此卦者…」一句没有标点，
    紧接着卦名 + 象征/是/的本意/代表/卦。在接缝处补「。」。"""
    return re.sub(
        r"(得此卦者[^，。；：！？\n]{1,24})(" + re.escape(name) + r"(?:象征|是|的本意|的意思|代表|卦))",
        r"\1。\2", text, count=1)


def line_name(pos: int, bit: int) -> str:
    n = "九" if bit else "六"
    return POSITIONS[pos] + n if pos in (0, 5) else n + POSITIONS[pos]


def parse_legacy(legacy: dict) -> dict[int, dict]:
    """旧 hexagram.json -> {序号: {name, pinyin, full_name, interpretation}}"""
    out: dict[int, dict] = {}
    pat = re.compile(r"第(\d+)卦\s*(.+?)（(.+?)）：(.+)")
    for entry in legacy.values():
        m = pat.match(entry["name"].strip())
        if not m:
            raise SystemExit(f"无法解析旧卦名: {entry['name']!r}")
        out[int(m.group(1))] = {
            "name": m.group(2),
            "pinyin": m.group(3),
            "full_name": m.group(4),
            "interpretation": entry["interpretation"].strip(),  # 接缝修复在 parse 之后按卦名进行
        }
    return out


def main() -> int:
    src = json.loads((SRC / "open_iching.json").read_text(encoding="utf-8"))
    xiang = json.loads((SRC / "open_iching_xiang.json").read_text(encoding="utf-8"))
    legacy = parse_legacy(json.loads((SRC / "legacy_hexagram.json").read_text(encoding="utf-8")))

    hexagrams: dict[str, dict] = {}
    for h in src:
        hid = h["id"]
        code = "".join(str(b) for b in h["array"])  # 自下而上，1=阳
        if code in hexagrams:
            raise SystemExit(f"卦码重复: {code}")
        lower, upper = h["combination"]
        old = legacy.get(hid)
        if old is not None:
            old = {**old, "interpretation": fix_seams(old["interpretation"], old["name"])}
        if old is None and hid == 20:
            old = {**GUAN_META, "name": h["name"], "interpretation": GUAN_INTERPRETATION}
        if old is None:
            raise SystemExit(f"第 {hid} 卦缺少白话/拼音数据")

        lines, extra = [], None
        for ln in h["lines"]:
            key = f"iching__{hid}_{ln['id']}"
            if ln["id"] <= 6:
                pos = ln["id"] - 1
                if ln["name"] != line_name(pos, ln["type"]) or ln["type"] != h["array"][pos]:
                    raise SystemExit(f"第 {hid} 卦爻名/阴阳不一致: {ln}")
                lines.append({"pos": ln["id"], "name": ln["name"],
                              "text": ln["scripture"], "xiang": xiang[key]})
            else:  # 用九 / 用六
                extra = {"name": ln["name"], "text": ln["scripture"], "xiang": xiang[key]}
        if len(lines) != 6:
            raise SystemExit(f"第 {hid} 卦爻数不是 6")

        entry = {
            "id": hid,
            "name": old["name"],
            "full_name": old["full_name"],
            "pinyin": old["pinyin"],
            "symbol": h["symbol"],
            "lower": lower,
            "upper": upper,
            "judgement": h["scripture"],
            "xiang_image": xiang[f"iching__{hid}"],
            "interpretation": old["interpretation"],
            "lines": lines,
        }
        if extra:
            entry["extra"] = extra
        hexagrams[code] = entry

    if len(hexagrams) != 64:
        raise SystemExit(f"卦数不是 64: {len(hexagrams)}")
    n_lines = sum(len(e["lines"]) + (1 if "extra" in e else 0) for e in hexagrams.values())
    if n_lines != 386:
        raise SystemExit(f"爻辞条数不是 386: {n_lines}")

    data = {
        "meta": {
            "schema": 2,
            "key": "卦码：6 位 0/1 字符串，自初爻到上爻（自下而上），1=阳、0=阴",
            "counts": {"hexagrams": 64, "lines": 384, "extra(用九/用六)": 2},
            "sources": [
                "卦辞、爻辞、象传：《周易》经传原文（古籍），整理自 github.com/john-walks-slow/open-iching",
                "卦辞白话 interpretation：沿用旧版 hexagram.json；第 20 卦「观」为新写",
            ],
            "notes": [
                "旧版把「剥」错标为 000011（观的卦码），已按序号修正",
                "爻辞无白话，旧版 yao.json 仅 12 条且并非逐卦逐爻，已弃用",
                "旧白话在『摘要｜详解』接缝处漏标句号（27 处），已补「。」",
            ],
        },
        "hexagrams": dict(sorted(hexagrams.items(), key=lambda kv: kv[1]["id"])),
    }
    OUT.parent.mkdir(parents=True, exist_ok=True)
    text = json.dumps(data, ensure_ascii=False, indent=1) + "\n"
    OUT.write_text(text, encoding="utf-8")
    GO_OUT.parent.mkdir(parents=True, exist_ok=True)
    GO_OUT.write_text(text, encoding="utf-8")
    print(f"写入 {OUT.relative_to(ROOT)} 与 {GO_OUT.relative_to(ROOT)}: {len(hexagrams)} 卦, {n_lines} 条爻辞（含用九/用六）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
