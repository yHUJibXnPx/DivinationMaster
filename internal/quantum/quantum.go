// Package quantum 是六比特态向量引擎（纯标准库，经典计算机上的精确模拟）。
//
// 六爻 = 六个量子比特，64 卦 = 64 个计算基态。流程：制备 → 纠缠 → 干涉 → 测量。
//
//   - 制备：稳定爻（少阳/少阴）处于确定态（Fuzz>0 时带小涨落）；变爻（老阳/老阴）是
//     "本态 ↔ 变态"各 50% 的叠加，翻转分量的相位由 SHAKE256(种子|问题) 决定。
//   - 纠缠：应位爻对（初-四、二-五、三-上）上的受控相位门。只有两个应位爻都处于叠加态
//     （同为变爻，或 Fuzz>0）时才会真正产生纠缠。
//   - 干涉：对处于叠加态的比特做 Rx，把相位转成概率差异（CPhase 是对角门，单独不改变
//     测量概率）。确定态的爻不参与，因此默认情况下不变的爻绝对稳定，测量结果恒落在
//     "本卦 → 变卦"的路径上，与传统断卦一致。
//   - 测量：按玻恩规则抽样，坍缩为一个具体的卦。
//
// 诚实说明：这只是对六比特系统的数学模拟，不增加任何预测能力。
package quantum

import (
	"crypto/sha3"
	"fmt"
	"math"
	"math/cmplx"
	"sort"

	"divinationmaster/internal/hexagram"
	"divinationmaster/internal/rng"
)

const dim = hexagram.Dim

// YingPairs 应位：初-四、二-五、三-上（0 起的爻位）。
var YingPairs = [3][2]int{{0, 3}, {1, 4}, {2, 5}}

// Config 引擎参数。
type Config struct {
	Fuzz      float64 // 稳定爻的涨落角（弧度）：0 = 绝对稳定（默认），上限 π/4（与变爻同样不确定）
	Entangle  float64 // 应位 CPhase 角，0 = 不纠缠
	Interfere float64 // Rx 干涉角，只作用于处于叠加态的比特；0 = 不干涉
}

// DefaultConfig 返回默认参数：不变的爻绝对稳定，应位纠缠 π/2，干涉角 0.35。
// 需要"所有爻都带涨落"的探索性用法，可把 Fuzz 设为 0.2（旧版默认值）。
func DefaultConfig() Config {
	return Config{Fuzz: 0, Entangle: math.Pi / 2, Interfere: 0.35}
}

// Validate 校验参数。
func (c Config) Validate() error {
	for _, kv := range []struct {
		name string
		v    float64
	}{{"fuzz", c.Fuzz}, {"entangle", c.Entangle}, {"interfere", c.Interfere}} {
		if math.IsNaN(kv.v) || math.IsInf(kv.v, 0) {
			return fmt.Errorf("%s 必须是有限实数，收到 %v", kv.name, kv.v)
		}
	}
	if c.Fuzz < 0 || c.Fuzz > math.Pi/4 {
		return fmt.Errorf("fuzz 必须在 [0, π/4] 内（π/4 即与变爻同样不确定）")
	}
	return nil
}

// Matrix2 是 2×2 复矩阵。
type Matrix2 [2][2]complex128

// Rx 返回绕 X 轴旋转 beta 的单比特门。
func Rx(beta float64) Matrix2 {
	c, s := math.Cos(beta/2), -math.Sin(beta/2)
	return Matrix2{{complex(c, 0), complex(0, s)}, {complex(0, s), complex(c, 0)}}
}

// State 是 64 维复态向量。值类型：每个门返回新的 State。
type State [dim]complex128

// Basis 返回计算基态 |code⟩。
func Basis(code string) (State, error) {
	var s State
	idx, err := hexagram.CodeToIndex(code)
	if err != nil {
		return s, err
	}
	s[idx] = 1
	return s, nil
}

func abs2(a complex128) float64 { return real(a)*real(a) + imag(a)*imag(a) }

// Norm2 返回 ‖ψ‖²。
func (s State) Norm2() float64 {
	t := 0.0
	for _, a := range s {
		t += abs2(a)
	}
	return t
}

// Probabilities 返回 64 个基态的测量概率（已归一化）。
func (s State) Probabilities() [dim]float64 {
	var p [dim]float64
	total := s.Norm2()
	for i, a := range s {
		p[i] = abs2(a) / total
	}
	return p
}

// Entropy 返回测量结果分布的香农熵（比特），范围 0..6。
func (s State) Entropy() float64 {
	e := 0.0
	for _, p := range s.Probabilities() {
		if p > 0 {
			e -= p * math.Log2(p)
		}
	}
	return e
}

// Support 返回概率大于 eps 的基态下标（升序）。
func (s State) Support(eps float64) []int {
	out := []int{}
	for i, p := range s.Probabilities() {
		if p > eps {
			out = append(out, i)
		}
	}
	return out
}

// Ranked 是 (下标, 概率) 对。
type Ranked struct {
	Index int
	P     float64
}

// Top 返回概率最大的 k 个基态。排序键把概率量化到 1e-12 再按下标打破平局，
// 使数学上相等的概率在不同平台上排序一致。
func (s State) Top(k int) []Ranked {
	probs := s.Probabilities()
	order := make([]int, dim)
	for i := range order {
		order[i] = i
	}
	key := func(i int) float64 { return math.Floor(probs[i]*1e12 + 0.5) }
	sort.Slice(order, func(a, b int) bool {
		ka, kb := key(order[a]), key(order[b])
		if ka != kb {
			return ka > kb
		}
		return order[a] < order[b]
	})
	if k > dim {
		k = dim
	}
	out := make([]Ranked, k)
	for i := 0; i < k; i++ {
		out[i] = Ranked{order[i], probs[order[i]]}
	}
	return out
}

// LineEntanglement 返回第 q 个比特（0 起）与其余五个比特的纠缠熵（比特，0..1）。
func (s State) LineEntanglement(q int) float64 {
	bit := 1 << q
	var r00, r11 float64
	var r01 complex128
	for i := 0; i < dim; i++ {
		if i&bit != 0 {
			continue
		}
		a0, a1 := s[i], s[i|bit]
		r00 += abs2(a0)
		r11 += abs2(a1)
		r01 += a0 * cmplx.Conj(a1)
	}
	total := r00 + r11
	r00, r11 = r00/total, r11/total
	r01 /= complex(total, 0)
	root := math.Sqrt(math.Max((r00-r11)*(r00-r11)+4*cmplx.Abs(r01)*cmplx.Abs(r01), 0))
	e := 0.0
	for _, lam := range []float64{(1 + root) / 2, (1 - root) / 2} {
		if lam > 1e-15 {
			e -= lam * math.Log2(lam)
		}
	}
	return e
}

// Apply1Q 对第 q 个比特作用单比特门。
func (s State) Apply1Q(q int, m Matrix2) State {
	bit := 1 << q
	out := s
	for i := 0; i < dim; i++ {
		if i&bit != 0 {
			continue
		}
		a0, a1 := s[i], s[i|bit]
		out[i] = m[0][0]*a0 + m[0][1]*a1
		out[i|bit] = m[1][0]*a0 + m[1][1]*a1
	}
	return out
}

// ApplyCPhase 对比特 a、b 作用受控相位门：|11⟩ 分量乘 e^{iθ}。
func (s State) ApplyCPhase(a, b int, theta float64) State {
	mask := (1 << a) | (1 << b)
	phase := complex(math.Cos(theta), math.Sin(theta))
	out := s
	for i := 0; i < dim; i++ {
		if i&mask == mask {
			out[i] = s[i] * phase
		}
	}
	return out
}

// Permute 作用置换：new[perm[i]] = old[i]。
func (s State) Permute(perm *[dim]int) State {
	var out State
	for i, a := range s {
		out[perm[i]] = a
	}
	return out
}

// Cuo 错卦算子（酉）。
func (s State) Cuo() State { return s.Permute(&hexagram.PermCuo) }

// Zong 综卦算子（酉）。
func (s State) Zong() State { return s.Permute(&hexagram.PermZong) }

// Measure 按玻恩规则抽样，返回坍缩后的基态下标。
func (s State) Measure(r *rng.Rng) int {
	x := r.Float64()
	probs := s.Probabilities()
	acc := 0.0
	for i, p := range probs {
		acc += p
		if x < acc {
			return i
		}
	}
	best := 0 // 浮点舍入兜底
	for i, p := range probs {
		if p > probs[best] {
			best = i
		}
	}
	return best
}

// LinePhases 由 (种子, 问题) 确定性地导出六个相位（SHAKE256）。
func LinePhases(question string, seed uint64) [hexagram.NLines]float64 {
	digest := sha3.SumSHAKE256([]byte(fmt.Sprintf("%d|%s", seed, question)), hexagram.NLines)
	var ph [hexagram.NLines]float64
	for i, b := range digest {
		ph[i] = 2 * math.Pi * float64(b) / 256
	}
	return ph
}

// Prepare 制备积态：稳定爻几乎确定（涨落角 Fuzz），变爻本态/变态各 50%（π/4），
// 翻转分量带由 (种子, 问题) 决定的相位。
func Prepare(lines hexagram.Lines, question string, seed uint64, cfg Config) State {
	phases := LinePhases(question, seed)
	var stable [hexagram.NLines]int
	var keep [hexagram.NLines]float64
	var flip [hexagram.NLines]complex128
	for k, v := range lines {
		stable[k] = hexagram.LineBit(v)
		angle := cfg.Fuzz
		if hexagram.IsChanging(v) {
			angle = math.Pi / 4
		}
		keep[k] = math.Cos(angle)
		sn := math.Sin(angle)
		flip[k] = complex(sn*math.Cos(phases[k]), sn*math.Sin(phases[k]))
	}
	var st State
	for idx := 0; idx < dim; idx++ {
		amp := complex(1, 0)
		for k := 0; k < hexagram.NLines; k++ {
			if (idx>>k)&1 == stable[k] {
				amp *= complex(keep[k], 0)
			} else {
				amp *= flip[k]
			}
		}
		st[idx] = amp
	}
	return st
}

// Active 返回处于叠加态的比特：变爻；Fuzz>0 时则是全部爻。
func Active(lines hexagram.Lines, cfg Config) [hexagram.NLines]bool {
	var a [hexagram.NLines]bool
	for k, v := range lines {
		a[k] = hexagram.IsChanging(v) || cfg.Fuzz > 0
	}
	return a
}

// Evolve 纠缠（应位 CPhase）→ 干涉（只对 active 比特做 Rx）。
func Evolve(s State, cfg Config, active [hexagram.NLines]bool) State {
	for _, p := range YingPairs {
		s = s.ApplyCPhase(p[0], p[1], cfg.Entangle)
	}
	gate := Rx(cfg.Interfere)
	for q := 0; q < hexagram.NLines; q++ {
		if active[q] {
			s = s.Apply1Q(q, gate)
		}
	}
	return s
}

// Run = Prepare + Evolve。
func Run(lines hexagram.Lines, question string, seed uint64, cfg Config) State {
	return Evolve(Prepare(lines, question, seed, cfg), cfg, Active(lines, cfg))
}
