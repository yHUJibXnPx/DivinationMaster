"""一次完整占卜：起卦 → 量子态演化 → 测量 → 断卦。返回可直接写入 JSON 的 dict。

同一 (问题, 种子, 引擎参数) 的结果完全可复现；种子默认取自操作系统熵。
"""
from __future__ import annotations

import secrets
from datetime import datetime

from .corpus import IChing, load
from .hexagram import (
    N_LINES, changed_code_of, changing_positions, code_of, cuo, hu, index_to_code, zong,
)
from .quantum import EngineConfig, run_engine
from .reading import build_reading, cast_hexagram, observation_relation
from .rng import Rng


def new_seed() -> int:
    return secrets.randbits(64)


def divine(question: str, seed: int | None = None, cfg: EngineConfig | None = None,
           iching: IChing | None = None, top_k: int = 5) -> dict:
    iching = iching or load()
    cfg = cfg or EngineConfig()
    seed = new_seed() if seed is None else seed
    rng = Rng(seed)  # 同时校验种子范围 [0, 2^64)

    lines = cast_hexagram(rng)
    ben = code_of(lines)
    changing = changing_positions(lines)
    bian = changed_code_of(lines) if changing else None

    state = run_engine(lines, question, seed, cfg)
    measured = index_to_code(state.measure(rng))

    def ref(code: str) -> dict:
        return {"code": code, "id": iching.get(code)["id"], "name": iching.get(code)["name"]}

    return {
        "time": datetime.now().isoformat(timespec="seconds"),
        "question": question,
        "seed": seed,
        "config": {"fuzz": cfg.fuzz, "entangle": cfg.entangle, "interfere": cfg.interfere},
        "lines": lines,
        "changing": changing,
        "benguo": ref(ben),
        "biangua": ref(bian) if bian else None,
        "derived": {  # 本卦的传统卦变：错/综为酉变换，互为经典映射
            "cuo": ref(cuo(ben)), "zong": ref(zong(ben)), "hu": ref(hu(ben)),
        },
        "reading": build_reading(lines, iching),
        "quantum": {
            "entropy_bits": state.entropy(),
            "line_entanglement": [state.line_entanglement(q) for q in range(N_LINES)],
            "top": [{**ref(index_to_code(i)), "p": p} for i, p in state.top(top_k)],
            "measured": ref(measured),
            "relation": observation_relation(measured, lines),
        },
    }
