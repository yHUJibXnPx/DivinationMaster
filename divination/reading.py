"""起卦（三枚铜钱）与断卦规则（朱熹《易学启蒙》通行读法）。"""
from __future__ import annotations

from typing import Sequence

from .corpus import IChing
from .rng import Rng
from .hexagram import (
    OLD_YANG, OLD_YIN, N_LINES, changed_code_of, changing_positions, code_of,
)


def cast_line(rng: Rng) -> int:
    """三枚铜钱：正面（阳）记 3、反面（阴）记 2，和为 6/7/8/9，概率 1:3:3:1。"""
    return sum(2 + rng.coin() for _ in range(3))


def cast_hexagram(rng: Rng) -> list[int]:
    """自初爻起依次抛出六爻。"""
    return [cast_line(rng) for _ in range(N_LINES)]


def _passage(source: str, kind: str, label: str, text: str, xiang: str, primary: bool) -> dict:
    return {"source": source, "kind": kind, "label": label, "text": text,
            "xiang": xiang, "primary": primary}


def build_reading(lines: Sequence[int], iching: IChing) -> dict:
    """按变爻数量选取应读的经文。

    0 变：本卦卦辞            1 变：本卦变爻爻辞
    2 变：本卦两变爻爻辞，以上爻为主
    3 变：本卦卦辞 + 变卦卦辞，以本卦为主
    4 变：变卦两个不变爻的爻辞，以下爻为主
    5 变：变卦唯一不变爻的爻辞
    6 变：乾坤用九/用六，其余读变卦卦辞
    """
    ben_code = code_of(lines)
    ben = iching.get(ben_code)
    changing = changing_positions(lines)
    n = len(changing)
    bian_code = changed_code_of(lines) if n else None
    bian = iching.get(bian_code) if bian_code else None

    def judgement(src: str, h: dict, primary: bool) -> dict:
        return _passage(src, "卦辞", h["name"], h["judgement"], h["xiang_image"], primary)

    def yao(src: str, h: dict, pos: int, primary: bool) -> dict:
        ln = h["lines"][pos - 1]
        return _passage(src, "爻辞", ln["name"], ln["text"], ln["xiang"], primary)

    passages: list[dict]
    if n == 0:
        rule = "无变爻：以本卦卦辞为断"
        passages = [judgement("本卦", ben, True)]
    elif n == 1:
        rule = "一爻变：以本卦变爻的爻辞为断"
        passages = [yao("本卦", ben, changing[0], True)]
    elif n == 2:
        rule = "二爻变：读本卦两个变爻的爻辞，以上爻为主"
        passages = [yao("本卦", ben, changing[0], False), yao("本卦", ben, changing[1], True)]
    elif n == 3:
        rule = "三爻变：本卦卦辞与变卦卦辞合参，以本卦为主"
        passages = [judgement("本卦", ben, True), judgement("变卦", bian, False)]
    elif n == 4:
        rest = [p for p in range(1, N_LINES + 1) if p not in changing]
        rule = "四爻变：读变卦中两个不变爻的爻辞，以下爻为主"
        passages = [yao("变卦", bian, rest[0], True), yao("变卦", bian, rest[1], False)]
    elif n == 5:
        rest = [p for p in range(1, N_LINES + 1) if p not in changing][0]
        rule = "五爻变：以变卦中唯一不变爻的爻辞为断"
        passages = [yao("变卦", bian, rest, True)]
    else:
        extra = ben.get("extra")
        if extra and all(v in (OLD_YANG, OLD_YIN) for v in lines) and ben_code in ("111111", "000000"):
            rule = f"六爻皆变：乾坤两卦读「{extra['name']}」"
            passages = [_passage("本卦", "爻辞", extra["name"], extra["text"], extra["xiang"], True)]
        else:
            rule = "六爻皆变：以变卦卦辞为断"
            passages = [judgement("变卦", bian, True)]

    return {"rule": rule, "n_changing": n, "passages": passages}


def observation_relation(measured_code: str, lines: Sequence[int]) -> str:
    """测量得到的卦与"本卦 → 变卦"路径的关系。"""
    ben = code_of(lines)
    changing = set(changing_positions(lines))
    diff = {i + 1 for i in range(N_LINES) if measured_code[i] != ben[i]}
    if not diff:
        return "与本卦一致"
    if changing and diff == changing:
        return "与变卦一致"
    if diff <= changing:
        return f"本卦→变卦路径上的中间态（已变 {len(diff)}/{len(changing)} 个变爻）"
    return f"涨落态（偏离本卦 {len(diff)} 爻，含非变爻）"
