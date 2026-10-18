package core

import (
	"encoding/json"
	"testing"

	"divinationmaster/internal/corpus"
	"divinationmaster/internal/quantum"
)

func TestDivineIsReproducible(t *testing.T) {
	ic := corpus.MustLoad()
	a, _ := Divine("同一个问题", 42, quantum.DefaultConfig(), ic, 5)
	b, _ := Divine("同一个问题", 42, quantum.DefaultConfig(), ic, 5)
	a.Time, b.Time = "", ""
	if !equalJSON(a, b) {
		t.Error("同问题 + 同种子 + 同参数应得到同一结果")
	}
}

func TestDefaultMeasurementAlwaysOnChangePath(t *testing.T) {
	// 设计承诺：默认配置下测量结果不会偏离本卦→变卦路径，与传统断卦不矛盾
	ic := corpus.MustLoad()
	for seed := uint64(0); seed < 1500; seed++ {
		r, err := Divine("q", seed, quantum.DefaultConfig(), ic, 5)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.Quantum.Relation; len(got) >= 3 && got[:3] == "涨" {
			t.Fatalf("seed %d: 默认配置出现涨落态: %s", seed, got)
		}
	}
}

func TestTopListHasNoImpossibleEntries(t *testing.T) {
	ic := corpus.MustLoad()
	for seed := uint64(0); seed < 500; seed++ {
		r, _ := Divine("q", seed, quantum.DefaultConfig(), ic, 8)
		if len(r.Quantum.Top) == 0 {
			t.Fatalf("seed %d: Top 为空", seed)
		}
		prev := 2.0
		for _, e := range r.Quantum.Top {
			// 数学上相等的概率在末位有浮点噪声，Top 把它们量化后按下标排序，故降序只在 1e-12 容差内成立
			if e.P <= 1e-12 || e.P > prev+1e-12 {
				t.Fatalf("seed %d: Top 含零概率项或未降序: %+v", seed, r.Quantum.Top)
			}
			prev = e.P
		}
	}
	// 静卦（seed 25 无变爻）应只有一项，概率为 1
	r, _ := Divine("静卦", 25, quantum.DefaultConfig(), ic, 5)
	if len(r.Changing) != 0 || len(r.Quantum.Top) != 1 || r.Quantum.Top[0].P < 1-1e-12 {
		t.Errorf("静卦应只有一个确定的结果: %+v", r.Quantum.Top)
	}
}

func TestRejectsBadConfigAndAcceptsEdgeSeeds(t *testing.T) {
	ic := corpus.MustLoad()
	if _, err := Divine("q", 1, quantum.Config{Fuzz: 9}, ic, 5); err == nil {
		t.Error("非法配置应报错")
	}
	for _, seed := range []uint64{0, 1<<63 - 1, 1 << 63, ^uint64(0)} {
		if _, err := Divine("q", seed, quantum.DefaultConfig(), ic, 5); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
	}
}

func equalJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
