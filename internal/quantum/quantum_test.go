package quantum

import (
	"math"
	"testing"

	"divinationmaster/internal/hexagram"
	"divinationmaster/internal/reading"
	"divinationmaster/internal/rng"
)

var cfg0 = Config{Fuzz: 0, Entangle: math.Pi / 2, Interfere: 0}

func idx(code string) int { i, _ := hexagram.CodeToIndex(code); return i }

func TestNormalised(t *testing.T) {
	for _, l := range []hexagram.Lines{{7, 8, 7, 8, 7, 8}, {9, 6, 9, 6, 9, 6}, {9, 7, 7, 6, 8, 7}} {
		st := Run(l, "q", 1, DefaultConfig())
		if math.Abs(st.Norm2()-1) > 1e-12 {
			t.Errorf("norm2 = %v", st.Norm2())
		}
	}
}

func TestDeterministicWithoutChange(t *testing.T) {
	l := hexagram.Lines{7, 8, 7, 8, 7, 8}
	st := Run(l, "q", 1, cfg0)
	sup := st.Support(1e-12)
	if len(sup) != 1 || sup[0] != idx(hexagram.CodeOf(l)) || st.Entropy() > 1e-12 {
		t.Errorf("support=%v entropy=%v", sup, st.Entropy())
	}
}

func TestSupportIsSubcubeOfChangingLines(t *testing.T) {
	l := hexagram.Lines{9, 7, 6, 8, 9, 7}
	st := Run(l, "q", 5, cfg0)
	ben := hexagram.CodeOf(l)
	var changing []int
	for i, v := range l {
		if hexagram.IsChanging(v) {
			changing = append(changing, i)
		}
	}
	expected := map[int]bool{}
	for mask := 0; mask < 1<<len(changing); mask++ {
		c := []byte(ben)
		for j, pos := range changing {
			if mask>>j&1 == 1 {
				c[pos] ^= 1
			}
		}
		expected[idx(string(c))] = true
	}
	sup := st.Support(1e-12)
	if len(sup) != len(expected) {
		t.Fatalf("support 大小 %d，应为 %d", len(sup), len(expected))
	}
	probs := st.Probabilities()
	for _, i := range sup {
		if !expected[i] || math.Abs(probs[i]-1/float64(len(expected))) > 1e-12 {
			t.Errorf("基态 %d 不在子立方体内或概率不均等: %v", i, probs[i])
		}
	}
	if math.Abs(st.Entropy()-float64(len(changing))) > 1e-9 {
		t.Errorf("熵 = %v，应为 %d", st.Entropy(), len(changing))
	}
}

func TestEntanglement(t *testing.T) {
	zero := Run(hexagram.Lines{9, 6, 9, 6, 9, 6}, "q", 3, Config{Fuzz: 0.2, Entangle: 0, Interfere: 0.35})
	for q := 0; q < 6; q++ {
		if zero.LineEntanglement(q) > 1e-9 {
			t.Errorf("entangle=0 时第 %d 爻纠缠熵 = %v", q, zero.LineEntanglement(q))
		}
	}
	l := hexagram.Lines{9, 7, 7, 6, 7, 7} // 初爻与四爻互为应位且都在叠加态
	a := Run(l, "q", 3, Config{Fuzz: 0, Entangle: math.Pi / 2, Interfere: 0})
	b := Run(l, "q", 3, Config{Fuzz: 0, Entangle: math.Pi / 2, Interfere: 0.7})
	for _, q := range []int{0, 3} {
		if a.LineEntanglement(q) <= 0.5 {
			t.Errorf("应位爻 %d 应纠缠，熵 = %v", q, a.LineEntanglement(q))
		}
	}
	for _, q := range []int{1, 2, 4, 5} {
		if a.LineEntanglement(q) > 1e-9 {
			t.Errorf("稳定爻 %d 不应纠缠，熵 = %v", q, a.LineEntanglement(q))
		}
	}
	for q := 0; q < 6; q++ { // 局域干涉门不改变纠缠熵
		if math.Abs(a.LineEntanglement(q)-b.LineEntanglement(q)) > 1e-9 {
			t.Errorf("局域门改变了第 %d 爻纠缠熵", q)
		}
	}
}

func TestQuestionOnlyMattersThroughInterference(t *testing.T) {
	l := hexagram.Lines{9, 7, 7, 6, 7, 7}
	maxDiff := func(a, b State) float64 {
		pa, pb, m := a.Probabilities(), b.Probabilities(), 0.0
		for i := range pa {
			m = math.Max(m, math.Abs(pa[i]-pb[i]))
		}
		return m
	}
	if d := maxDiff(Run(l, "甲", 11, DefaultConfig()), Run(l, "乙", 11, DefaultConfig())); d < 1e-4 {
		t.Errorf("有干涉时问题应改变分布，最大差 = %v", d)
	}
	if d := maxDiff(Run(l, "甲", 11, cfg0), Run(l, "乙", 11, cfg0)); d > 1e-12 {
		t.Errorf("无干涉时相位不应改变概率，最大差 = %v", d)
	}
}

func TestCuoZongUnitary(t *testing.T) {
	st := Run(hexagram.Lines{9, 7, 8, 6, 7, 8}, "q", 4, DefaultConfig())
	for _, out := range []State{st.Cuo(), st.Zong()} {
		if math.Abs(out.Norm2()-1) > 1e-12 {
			t.Error("算子改变了范数")
		}
	}
	if st.Cuo().Cuo() != st || st.Zong().Zong() != st {
		t.Error("错/综应是对合")
	}
	b, _ := Basis("100010")
	if sup := b.Zong().Support(0); len(sup) != 1 || sup[0] != idx("010001") {
		t.Errorf("屯综应为蒙，得 %v", sup)
	}
}

func TestMeasureFollowsBornRule(t *testing.T) {
	st := Prepare(hexagram.Lines{9, 7, 7, 7, 7, 7}, "q", 1, Config{Fuzz: 0})
	r := rng.New(123)
	const n = 4000
	hits := 0
	for i := 0; i < n; i++ {
		if st.Measure(r) == idx("111111") {
			hits++
		}
	}
	if f := float64(hits) / n; math.Abs(f-0.5) > 0.04 {
		t.Errorf("频率 %v，应约为 0.5", f)
	}
}

func TestTopTieBreakIsDeterministic(t *testing.T) {
	// 无涨落/无干涉时 2^k 个卦概率完全相等：排序必须按下标打破平局
	st := Run(hexagram.Lines{9, 6, 7, 7, 7, 7}, "q", 1, cfg0)
	top := st.Top(4)
	for i := 1; i < len(top); i++ {
		if top[i].Index <= top[i-1].Index {
			t.Errorf("平局未按下标升序: %v", top)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	bad := []Config{{Fuzz: -0.1}, {Fuzz: 1.0}, {Entangle: math.NaN()}, {Interfere: math.Inf(1)}}
	for _, c := range bad {
		if c.Validate() == nil {
			t.Errorf("%+v 应被拒绝", c)
		}
	}
	if DefaultConfig().Validate() != nil {
		t.Error("默认配置应合法")
	}
}

// subcube 返回"变爻任意子集翻转"得到的全部卦的下标，即本卦→变卦路径所在的子立方体。
func subcube(lines hexagram.Lines) map[int]bool {
	ben := hexagram.CodeOf(lines)
	pos := hexagram.ChangingPositions(lines)
	cube := map[int]bool{}
	for mask := 0; mask < 1<<len(pos); mask++ {
		c := []byte(ben)
		for j, p := range pos {
			if mask>>j&1 == 1 {
				c[p-1] ^= 1
			}
		}
		cube[idx(string(c))] = true
	}
	return cube
}

func TestDefaultKeepsStableLinesStable(t *testing.T) {
	// 设计承诺：默认配置下，不变的爻绝对稳定——测量结果恒落在"本卦→变卦"的路径上，
	// 无变爻（静卦）时结果完全确定。
	r := rng.New(99)
	for i := 0; i < 2000; i++ {
		lines := reading.CastHexagram(r)
		st := Run(lines, "q", uint64(i), DefaultConfig())
		cube := subcube(lines)
		for _, k := range st.Support(1e-12) {
			if !cube[k] {
				t.Fatalf("lines=%v: 基态 %s 不在本卦→变卦路径上", lines, hexagram.IndexToCode(k))
			}
		}
		if len(hexagram.ChangingPositions(lines)) == 0 && st.Entropy() > 1e-12 {
			t.Fatalf("静卦 %v 应完全确定，熵 = %v", lines, st.Entropy())
		}
	}
}

func TestFuzzLetsStableLinesFluctuate(t *testing.T) {
	cfg := Config{Fuzz: 0.2, Entangle: math.Pi / 2, Interfere: 0.35}
	st := Run(hexagram.Lines{7, 8, 7, 8, 7, 8}, "q", 1, cfg)
	if n := len(st.Support(1e-12)); n <= 1 {
		t.Errorf("Fuzz>0 时静卦也应有涨落，support = %d", n)
	}
}

func TestActive(t *testing.T) {
	l := hexagram.Lines{9, 7, 6, 8, 7, 7}
	a := Active(l, Config{})
	want := [hexagram.NLines]bool{true, false, true, false, false, false}
	if a != want {
		t.Errorf("Fuzz=0: Active = %v, want %v", a, want)
	}
	for k, on := range Active(l, Config{Fuzz: 0.1}) {
		if !on {
			t.Errorf("Fuzz>0 时第 %d 爻也应处于叠加态", k)
		}
	}
}
