#!/usr/bin/env python3
"""生成 Python/Go 共用的"标准答案" testdata/golden.json 与 testdata/history_sample.jsonl。

Python 版是参考实现；Go 版（go/internal/parity）逐项对拍。改动引擎后需重新生成，
并同时通过两边的测试。用法（项目根目录）：
    python tools/gen_golden.py           重新生成
    python tools/gen_golden.py --check   只检查已提交的文件是否与当前 Python 实现一致（不写文件）
"""
from __future__ import annotations

import contextlib
import io
import json
import math
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

from divination import history  # noqa: E402
from divination.cli import main as cli_main  # noqa: E402
from divination.core import divine  # noqa: E402
from divination.quantum import EngineConfig, run_engine  # noqa: E402
from divination.reading import build_reading  # noqa: E402
from divination.corpus import load  # noqa: E402
from divination.render import render_history_row  # noqa: E402
from divination.rng import Rng  # noqa: E402

OUT = ROOT / "testdata"


def cfg_dict(c: EngineConfig) -> dict:
    return {"fuzz": c.fuzz, "entangle": c.entangle, "interfere": c.interfere}


def rng_cases() -> list[dict]:
    out = []
    for seed in (0, 1, 42, 12345678901234567890, 2**64 - 1):
        r = Rng(seed)
        u64 = [str(r.next_u64()) for _ in range(8)]
        r2 = Rng(seed)
        floats = [repr(r2.random()) for _ in range(4)]
        r3 = Rng(seed)
        coins = [r3.coin() for _ in range(16)]
        out.append({"seed": str(seed), "u64": u64, "floats": floats, "coins": coins})
    return out


ENGINE_LINES = [
    [7, 8, 7, 8, 7, 8], [9, 7, 7, 6, 7, 7], [9, 6, 9, 6, 9, 6], [9, 7, 6, 8, 9, 7],
    [9] * 6, [6] * 6, [8, 8, 8, 8, 8, 8], [9, 8, 7, 6, 8, 7], [6, 9, 7, 7, 8, 8],
]
ENGINE_CFGS = [
    EngineConfig(),
    EngineConfig(fuzz=0.0, entangle=0.0, interfere=0.0),
    EngineConfig(fuzz=0.0, entangle=math.pi / 2, interfere=0.0),
    EngineConfig(fuzz=0.5, entangle=1.0, interfere=0.9),
]


def engine_cases() -> list[dict]:
    out = []
    n = 0
    for lines in ENGINE_LINES:
        for cfg in ENGINE_CFGS:
            n += 1
            question, seed = f"引擎对拍{n}", 1000 + n
            st = run_engine(lines, question, seed, cfg)
            from divination.hexagram import index_to_code
            out.append({
                "lines": lines, "question": question, "seed": str(seed), "config": cfg_dict(cfg),
                "probabilities": st.probabilities(),
                "entropy": st.entropy(),
                "line_entanglement": [st.line_entanglement(q) for q in range(6)],
                "top": [[index_to_code(i), p] for i, p in st.top(5)],
                "measured": index_to_code(st.measure(Rng(seed))),
            })
    return out


READING_LINES = [
    [7, 8, 7, 8, 7, 8], [9, 7, 7, 7, 7, 7], [9, 7, 6, 7, 7, 7], [9, 9, 9, 7, 7, 7],
    [9, 9, 9, 9, 7, 7], [9, 9, 9, 9, 9, 7], [9] * 6, [6] * 6, [9, 6, 6, 6, 9, 6],
    [6, 8, 7, 9, 6, 8], [7, 7, 6, 6, 6, 6], [8, 9, 8, 9, 8, 9],
]


def reading_cases() -> list[dict]:
    ic = load()
    return [{"lines": ln, "reading": build_reading(ln, ic)} for ln in READING_LINES]


def divine_cases() -> list[dict]:
    out = []
    for i in range(30):
        cfg = EngineConfig() if i < 24 else [
            EngineConfig(fuzz=0.0, entangle=0.0, interfere=0.0),
            EngineConfig(fuzz=0.4, entangle=2.0, interfere=0.8),
            EngineConfig(fuzz=0.1, entangle=math.pi, interfere=0.2),
            EngineConfig(fuzz=0.0, entangle=math.pi / 2, interfere=0.0),
            EngineConfig(fuzz=0.3, entangle=0.0, interfere=0.5),
            EngineConfig(fuzz=0.2, entangle=0.7, interfere=0.0),
        ][i - 24]
        question = "" if i == 0 else f"第{i}个问题：我该怎么做？"
        seed = i * 7919 + 3 if i < 28 else 2**64 - 1 - i
        r = divine(question, seed=seed, cfg=cfg, top_k=5)
        r.pop("time")
        out.append(r)
    return out


CLI_ARGS = [
    ["我该接受这份工作吗？", "--seed", "7", "--no-save"],
    ["ask", "问题乙", "--seed", "12345", "--top", "3", "--no-save"],
    ["--seed", "9", "--fuzz", "0", "--entangle", "0", "--interfere", "0", "--no-save", "零参数"],
    ["ask", "", "--seed", "0", "--no-save"],
    ["ask", "大数种子", "--seed", "18446744073709551615", "--top", "8", "--no-save"],
    ["show", "屯"], ["show", "乾"], ["show", "坤"], ["show", "20"], ["show", "100010"], ["show", "水雷屯"],
    ["show", "不存在的卦"], ["ask", "q", "--fuzz", "5", "--no-save"],
]


def cli_cases() -> list[dict]:
    out = []
    for argv in CLI_ARGS:
        buf, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(err):
            try:
                code = cli_main(list(argv))
            except SystemExit as e:
                code = int(e.code or 0)
        out.append({"args": argv, "exit": code, "stdout": buf.getvalue()})
    return out


def history_sample() -> tuple[list[dict], list[str]]:
    recs = []
    for i, seed in enumerate((11, 22, 33)):
        r = divine(f"历史样例{i}", seed=seed)
        r["time"] = f"2026-10-17T08:0{i}:00"
        recs.append(r)
    return recs, [render_history_row(r) for r in recs]


def main(argv: list[str]) -> int:
    check = "--check" in argv
    recs, rows = history_sample()
    sample_text = "".join(json.dumps(r, ensure_ascii=False) + "\n" for r in recs)
    golden = {
        "note": "由 tools/gen_golden.py 生成；Python 为参考实现，Go 版对拍。勿手改。",
        "rng": rng_cases(), "engine": engine_cases(), "reading": reading_cases(),
        "divine": divine_cases(), "cli": cli_cases(), "history_rows": rows,
    }
    golden_text = json.dumps(golden, ensure_ascii=False, indent=1) + "\n"
    targets = {OUT / "history_sample.jsonl": sample_text, OUT / "golden.json": golden_text}
    if check:
        stale = [p.name for p, text in targets.items() if not p.exists() or p.read_text(encoding="utf-8") != text]
        if stale:
            print(f"标准答案已过期：{', '.join(stale)}。若是有意改动引擎，请运行 python tools/gen_golden.py 并让 Go 版通过对拍。",
                  file=sys.stderr)
            return 1
        print("标准答案与当前 Python 实现一致")
        return 0
    OUT.mkdir(exist_ok=True)
    for p, text in targets.items():
        p.write_text(text, encoding="utf-8")
    print(f"golden: rng={len(golden['rng'])} engine={len(golden['engine'])} reading={len(golden['reading'])} "
          f"divine={len(golden['divine'])} cli={len(golden['cli'])} history={len(rows)}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
