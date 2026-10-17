"""可跨语言复现的伪随机数发生器：SplitMix64 播种 + xoshiro256**。

为什么不用 random.Random：梅森旋转在 Go/Rust 标准库里没有等价物，
而本项目承诺"同问题 + 同种子 + 同参数 = 同结果"（Python 版与 Go 版一致）。
本模块只用整数运算，Go 版 internal/rng 逐步骤等价，并由 testdata/golden.json 对拍。

注意：这不是密码学随机数。种子默认取自操作系统熵（secrets.randbits(64)）。
"""
from __future__ import annotations

MASK = (1 << 64) - 1
SEED_MAX = MASK


def _rotl(x: int, k: int) -> int:
    return ((x << k) | (x >> (64 - k))) & MASK


def splitmix64(state: int) -> tuple[int, int]:
    """返回 (新状态, 输出)。"""
    state = (state + 0x9E3779B97F4A7C15) & MASK
    z = state
    z = ((z ^ (z >> 30)) * 0xBF58476D1CE4E5B9) & MASK
    z = ((z ^ (z >> 27)) * 0x94D049BB133111EB) & MASK
    return state, z ^ (z >> 31)


class Rng:
    __slots__ = ("_s",)

    def __init__(self, seed: int) -> None:
        if not 0 <= seed <= SEED_MAX:
            raise ValueError(f"种子必须在 [0, 2^64) 内，收到 {seed!r}")
        sm = seed
        s = []
        for _ in range(4):
            sm, out = splitmix64(sm)
            s.append(out)
        self._s = s

    def next_u64(self) -> int:
        s = self._s
        result = (_rotl((s[1] * 5) & MASK, 7) * 9) & MASK
        t = (s[1] << 17) & MASK
        s[2] ^= s[0]
        s[3] ^= s[1]
        s[1] ^= s[2]
        s[0] ^= s[3]
        s[2] ^= t
        s[3] = _rotl(s[3], 45)
        return result

    def random(self) -> float:
        """[0, 1) 上的均匀 double（取高 53 位）。"""
        return (self.next_u64() >> 11) * (1.0 / (1 << 53))

    def coin(self) -> int:
        """0 或 1（取最高位）。"""
        return self.next_u64() >> 63
