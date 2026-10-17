// Package rng 是可跨语言复现的伪随机数发生器：SplitMix64 播种 + xoshiro256**。
//
// 与 Python 版 divination/rng.py 逐步骤等价（只用整数运算），由 testdata/golden.json 对拍。
// 不是密码学随机数；种子默认取自操作系统熵（见 core.NewSeed）。
package rng

import "math/bits"

// Rng 是 xoshiro256** 发生器。
type Rng struct{ s [4]uint64 }

func splitmix64(state uint64) (uint64, uint64) {
	state += 0x9E3779B97F4A7C15
	z := state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return state, z ^ (z >> 31)
}

// New 用 64 位种子构造发生器（种子经 SplitMix64 展开为 256 位状态）。
func New(seed uint64) *Rng {
	r := &Rng{}
	sm := seed
	for i := range r.s {
		sm, r.s[i] = splitmix64(sm)
	}
	return r
}

// Uint64 返回下一个 64 位输出。
func (r *Rng) Uint64() uint64 {
	s := &r.s
	result := bits.RotateLeft64(s[1]*5, 7) * 9
	t := s[1] << 17
	s[2] ^= s[0]
	s[3] ^= s[1]
	s[1] ^= s[2]
	s[0] ^= s[3]
	s[2] ^= t
	s[3] = bits.RotateLeft64(s[3], 45)
	return result
}

// Float64 返回 [0,1) 上的均匀 float64（取高 53 位）。
func (r *Rng) Float64() float64 {
	return float64(r.Uint64()>>11) * (1.0 / (1 << 53))
}

// Coin 返回 0 或 1（取最高位）。
func (r *Rng) Coin() int {
	return int(r.Uint64() >> 63)
}
