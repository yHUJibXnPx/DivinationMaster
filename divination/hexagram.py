"""六爻、卦码与卦变算子（纯函数，无 I/O）。

约定
----
* 卦码：6 位 '0'/'1' 字符串，**自初爻到上爻**（自下而上），1=阳、0=阴。
  与旧版 hexagram.json 的键一致。
* 态向量下标：卦码反转后按二进制读出，即下标的第 i 位（低位起）= 第 i+1 爻。
* 四态爻（三枚铜钱之和）：6 老阴、7 少阳、8 少阴、9 老阳。
  老阴/老阳是"变爻"：当下为阴/阳，但正在变为阳/阴。
"""
from __future__ import annotations

from typing import Sequence

OLD_YIN, YOUNG_YANG, YOUNG_YIN, OLD_YANG = 6, 7, 8, 9
VALID_LINES = (OLD_YIN, YOUNG_YANG, YOUNG_YIN, OLD_YANG)
POSITIONS = ("初", "二", "三", "四", "五", "上")
N_LINES = 6
DIM = 1 << N_LINES  # 64


def line_bit(value: int) -> int:
    """四态爻 -> 本卦中的阴阳（1 阳 / 0 阴）。"""
    if value not in VALID_LINES:
        raise ValueError(f"爻值必须是 6/7/8/9，收到 {value!r}")
    return 1 if value in (YOUNG_YANG, OLD_YANG) else 0


def is_changing(value: int) -> bool:
    return value in (OLD_YIN, OLD_YANG)


def _check_lines(lines: Sequence[int]) -> None:
    if len(lines) != N_LINES:
        raise ValueError(f"需要 6 个爻，收到 {len(lines)} 个")


def check_code(code: str) -> str:
    if len(code) != N_LINES or set(code) - {"0", "1"}:
        raise ValueError(f"卦码必须是 6 位 0/1 字符串，收到 {code!r}")
    return code


def code_of(lines: Sequence[int]) -> str:
    """本卦卦码。"""
    _check_lines(lines)
    return "".join(str(line_bit(v)) for v in lines)


def changed_code_of(lines: Sequence[int]) -> str:
    """变卦卦码：变爻阴阳互换，其余不变。"""
    _check_lines(lines)
    return "".join(str(line_bit(v) ^ (1 if is_changing(v) else 0)) for v in lines)


def changing_positions(lines: Sequence[int]) -> list[int]:
    """变爻位置，1 起（1=初爻 … 6=上爻）。"""
    _check_lines(lines)
    return [i + 1 for i, v in enumerate(lines) if is_changing(v)]


def line_name(pos0: int, bit: int) -> str:
    """pos0 为 0 起的爻位；返回如 初九、六二、上六。"""
    n = "九" if bit else "六"
    return POSITIONS[pos0] + n if pos0 in (0, N_LINES - 1) else n + POSITIONS[pos0]


def code_to_index(code: str) -> int:
    return int(check_code(code)[::-1], 2)


def index_to_code(index: int) -> str:
    if not 0 <= index < DIM:
        raise ValueError(f"下标越界: {index}")
    return format(index, f"0{N_LINES}b")[::-1]


# ---- 传统的卦变关系 --------------------------------------------------------

def cuo(code: str) -> str:
    """错卦：六爻阴阳全部互换（可逆，酉算子 X⊗6）。"""
    return "".join("1" if c == "0" else "0" for c in check_code(code))


def zong(code: str) -> str:
    """综卦：上下颠倒（可逆，比特序反转的置换算子）。"""
    return check_code(code)[::-1]


def hu(code: str) -> str:
    """互卦：二三四爻为下卦、三四五爻为上卦。

    注意：这是多对一映射（64 -> 16），不可逆，因此**不是**酉算子，
    只能作用在测量结果（经典信息）上，不能作用在态向量上。
    """
    check_code(code)
    return code[1:4] + code[2:5]


# 作用在态向量下标上的置换（new[PERM[i]] = old[i]）
PERM_CUO = [code_to_index(cuo(index_to_code(i))) for i in range(DIM)]
PERM_ZONG = [code_to_index(zong(index_to_code(i))) for i in range(DIM)]


def render_lines(code: str, changing: Sequence[int] = ()) -> list[str]:
    """文本卦象，自上而下；changing 为 1 起的变爻位置，阳变用 ○、阴变用 ×。"""
    rows = []
    for pos0 in range(N_LINES - 1, -1, -1):
        yang = code[pos0] == "1"
        body = "━━━━━━━━" if yang else "━━━  ━━━"
        mark = ""
        if (pos0 + 1) in changing:
            mark = " ○" if yang else " ×"
        rows.append(f"{line_name(pos0, int(yang))}  {body}{mark}")
    return rows
