"""六比特态向量引擎（纯标准库，经典计算机上的精确模拟）。

模型：六爻 = 六个量子比特，64 卦 = 64 个计算基态。
    下标 i 的第 k 位 = 第 k+1 爻（见 hexagram.py），1 = 阳。

流程：制备 → 纠缠 → 干涉 → 测量
    制备  每一爻一个量子比特，组成积态：
            稳定爻（少阳/少阴）：几乎处于确定态，仅有角度 fuzz 的涨落；
            变爻（老阳/老阴）：本态与变态各 50%（角度 π/4）；
            翻转分量带一个相位 φ_k，由 sha3(种子|问题) 决定。
          —— 问题的措辞只通过这些相位影响结果，没有任何"语义"。
    纠缠  在"应位"爻对（初-四、二-五、三-上）上做受控相位门 CPhase(θ)。
    干涉  对每个比特做小角度 Rx(β)，把相位信息转成概率差异
          （CPhase 是对角门，单独不改变测量概率）。
    测量  按玻恩规则对 64 个基态抽样，坍缩为一个具体的卦。

诚实说明：这只是对"六比特系统"的数学模拟，不增加任何预测能力；
它的价值在于给出"可能性空间"的结构，而不是单点结论。
"""
from __future__ import annotations

import cmath
import hashlib
import math
from dataclasses import dataclass
from typing import Sequence

from .rng import Rng
from .hexagram import DIM, N_LINES, PERM_CUO, PERM_ZONG, code_to_index, is_changing, line_bit

# 应位：初-四、二-五、三-上（0 起的爻位）
YING_PAIRS = ((0, 3), (1, 4), (2, 5))

Matrix2 = tuple[tuple[complex, complex], tuple[complex, complex]]


@dataclass(frozen=True)
class EngineConfig:
    fuzz: float = 0.2                # 稳定爻的涨落角（弧度），0 = 绝对稳定
    entangle: float = math.pi / 2    # 应位 CPhase 角，0 = 不纠缠
    interfere: float = 0.35          # 末端 Rx 干涉角，0 = 不干涉

    def __post_init__(self) -> None:
        for name in ("fuzz", "entangle", "interfere"):
            v = getattr(self, name)
            if not math.isfinite(v):
                raise ValueError(f"{name} 必须是有限实数，收到 {v!r}")
        if not 0.0 <= self.fuzz <= math.pi / 4:
            raise ValueError("fuzz 必须在 [0, π/4] 内（π/4 即与变爻同样不确定）")


def rx(beta: float) -> Matrix2:
    c, s = math.cos(beta / 2), -1j * math.sin(beta / 2)
    return ((c, s), (s, c))


X_GATE: Matrix2 = ((0, 1), (1, 0))


class State:
    """64 维复态向量（不可变风格：每个门返回新对象）。"""

    __slots__ = ("amps", "_probs")

    def __init__(self, amps: Sequence[complex]) -> None:
        if len(amps) != DIM:
            raise ValueError(f"态向量长度必须是 {DIM}")
        self.amps = [complex(a) for a in amps]
        self._probs: list[float] | None = None  # 态不可变，概率只需算一次

    # ---- 构造 ----
    @classmethod
    def basis(cls, code: str) -> "State":
        amps = [0j] * DIM
        amps[code_to_index(code)] = 1 + 0j
        return cls(amps)

    # ---- 基本量 ----
    def norm2(self) -> float:
        return sum(a.real * a.real + a.imag * a.imag for a in self.amps)

    def probabilities(self) -> list[float]:
        if self._probs is None:
            total = self.norm2()
            self._probs = [(a.real * a.real + a.imag * a.imag) / total for a in self.amps]
        return list(self._probs)

    def top(self, k: int = 5) -> list[tuple[int, float]]:
        """概率最大的 k 个基态。排序键把概率量化到 1e-12：数学上相等的概率
        （如 2^k 个等概率卦）在不同平台上会有末位浮点噪声，量化后再按下标打破平局，
        使排序在 Python/Go、amd64/arm64 上一致。"""
        probs = self.probabilities()
        order = sorted(range(DIM), key=lambda i: (-int(probs[i] * 1e12 + 0.5), i))
        return [(i, probs[i]) for i in order[:k]]

    def support(self, eps: float = 1e-12) -> list[int]:
        return [i for i, p in enumerate(self.probabilities()) if p > eps]

    def entropy(self) -> float:
        """测量结果分布的香农熵（比特），范围 0..6。"""
        return -sum(p * math.log2(p) for p in self.probabilities() if p > 0)

    def line_entanglement(self, q: int) -> float:
        """第 q 个比特（0 起）与其余五个比特的纠缠熵（冯·诺依曼熵，比特，0..1）。"""
        bit = 1 << q
        r00 = r11 = 0.0
        r01 = 0j
        for i in range(DIM):
            if i & bit:
                continue
            a0, a1 = self.amps[i], self.amps[i | bit]
            r00 += abs(a0) ** 2
            r11 += abs(a1) ** 2
            r01 += a0 * a1.conjugate()
        total = r00 + r11
        r00, r11, r01 = r00 / total, r11 / total, r01 / total
        root = math.sqrt(max((r00 - r11) ** 2 + 4 * abs(r01) ** 2, 0.0))
        entropy = 0.0
        for lam in ((1 + root) / 2, (1 - root) / 2):
            if lam > 1e-15:
                entropy -= lam * math.log2(lam)
        return entropy

    # ---- 门 ----
    def apply_1q(self, q: int, m: Matrix2) -> "State":
        bit = 1 << q
        out = list(self.amps)
        for i in range(DIM):
            if i & bit:
                continue
            a0, a1 = self.amps[i], self.amps[i | bit]
            out[i] = m[0][0] * a0 + m[0][1] * a1
            out[i | bit] = m[1][0] * a0 + m[1][1] * a1
        return State(out)

    def apply_cphase(self, a: int, b: int, theta: float) -> "State":
        mask = (1 << a) | (1 << b)
        phase = cmath.exp(1j * theta)
        return State([amp * phase if (i & mask) == mask else amp for i, amp in enumerate(self.amps)])

    def permute(self, perm: Sequence[int]) -> "State":
        out = [0j] * DIM
        for i, amp in enumerate(self.amps):
            out[perm[i]] = amp
        return State(out)

    def cuo(self) -> "State":
        """错卦算子（酉）。"""
        return self.permute(PERM_CUO)

    def zong(self) -> "State":
        """综卦算子（酉）。"""
        return self.permute(PERM_ZONG)

    # ---- 测量 ----
    def measure(self, rng: Rng) -> int:
        """按玻恩规则抽样，返回坍缩后的基态下标。"""
        r = rng.random()
        acc = 0.0
        probs = self.probabilities()
        for i, p in enumerate(probs):
            acc += p
            if r < acc:
                return i
        return max(range(DIM), key=probs.__getitem__)  # 浮点舍入兜底


def line_phases(question: str, seed: int) -> list[float]:
    """由 (种子, 问题) 确定性地导出六个相位。"""
    digest = hashlib.shake_256(f"{seed}|{question}".encode("utf-8")).digest(N_LINES)
    return [2 * math.pi * b / 256 for b in digest]


def prepare(lines: Sequence[int], question: str, seed: int, cfg: EngineConfig) -> State:
    """制备积态。lines 为六个四态爻（6/7/8/9），自初爻起。"""
    if len(lines) != N_LINES:
        raise ValueError("需要 6 个爻")
    phases = line_phases(question, seed)
    stable_bit = [line_bit(v) for v in lines]
    angle = [math.pi / 4 if is_changing(v) else cfg.fuzz for v in lines]
    keep = [math.cos(a) for a in angle]
    flip = [math.sin(a) * cmath.exp(1j * phases[k]) for k, a in enumerate(angle)]
    amps = []
    for idx in range(DIM):
        amp = 1 + 0j
        for k in range(N_LINES):
            amp *= keep[k] if ((idx >> k) & 1) == stable_bit[k] else flip[k]
        amps.append(amp)
    return State(amps)


def evolve(state: State, cfg: EngineConfig) -> State:
    """纠缠（应位 CPhase）→ 干涉（逐比特 Rx）。"""
    for a, b in YING_PAIRS:
        state = state.apply_cphase(a, b, cfg.entangle)
    gate = rx(cfg.interfere)
    for q in range(N_LINES):
        state = state.apply_1q(q, gate)
    return state


def run_engine(lines: Sequence[int], question: str, seed: int, cfg: EngineConfig | None = None) -> State:
    cfg = cfg or EngineConfig()
    return evolve(prepare(lines, question, seed, cfg), cfg)
