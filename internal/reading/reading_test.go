package reading

import (
	"math"
	"testing"

	"divinationmaster/internal/corpus"
	"divinationmaster/internal/hexagram"
	"divinationmaster/internal/rng"
)

func TestCastDistribution(t *testing.T) {
	r := rng.New(2024)
	const n = 16000
	count := map[int]int{}
	for i := 0; i < n; i++ {
		count[CastLine(r)]++
	}
	want := map[int]float64{6: 1.0 / 8, 7: 3.0 / 8, 8: 3.0 / 8, 9: 1.0 / 8}
	if len(count) != 4 {
		t.Fatalf("爻值种类 %v", count)
	}
	for v, p := range want {
		if math.Abs(float64(count[v])/n-p) > 0.015 {
			t.Errorf("爻值 %d 频率 %v，期望约 %v", v, float64(count[v])/n, p)
		}
	}
}

func TestCastReproducible(t *testing.T) {
	if CastHexagram(rng.New(5)) != CastHexagram(rng.New(5)) {
		t.Error("同种子应得到同一卦")
	}
}

func TestRules(t *testing.T) {
	ic := corpus.MustLoad()
	type exp struct {
		lines   hexagram.Lines
		n       int
		sources []string
		kinds   []string
		labels  []string
		primary []bool
	}
	cases := []exp{
		{hexagram.Lines{7, 8, 7, 8, 7, 8}, 0, []string{"本卦"}, []string{"卦辞"}, []string{"既济"}, []bool{true}},
		{hexagram.Lines{9, 7, 7, 7, 7, 7}, 1, []string{"本卦"}, []string{"爻辞"}, []string{"初九"}, []bool{true}},
		{hexagram.Lines{9, 7, 6, 7, 7, 7}, 2, []string{"本卦", "本卦"}, []string{"爻辞", "爻辞"}, []string{"初九", "六三"}, []bool{false, true}},
		{hexagram.Lines{9, 9, 9, 7, 7, 7}, 3, []string{"本卦", "变卦"}, []string{"卦辞", "卦辞"}, nil, []bool{true, false}},
		{hexagram.Lines{9, 9, 9, 9, 7, 7}, 4, []string{"变卦", "变卦"}, []string{"爻辞", "爻辞"}, []string{"九五", "上九"}, []bool{true, false}},
		{hexagram.Lines{9, 9, 9, 9, 9, 7}, 5, []string{"变卦"}, []string{"爻辞"}, []string{"上九"}, []bool{true}},
		{hexagram.Lines{9, 9, 9, 9, 9, 9}, 6, []string{"本卦"}, []string{"爻辞"}, []string{"用九"}, []bool{true}},
		{hexagram.Lines{6, 6, 6, 6, 6, 6}, 6, []string{"本卦"}, []string{"爻辞"}, []string{"用六"}, []bool{true}},
		{hexagram.Lines{9, 6, 6, 6, 9, 6}, 6, []string{"变卦"}, []string{"卦辞"}, []string{"鼎"}, []bool{true}},
	}
	for _, c := range cases {
		r, err := Build(c.lines, ic)
		if err != nil {
			t.Fatal(err)
		}
		if r.NChanging != c.n || len(r.Passages) != len(c.sources) {
			t.Errorf("%v: n=%d passages=%d", c.lines, r.NChanging, len(r.Passages))
			continue
		}
		for i, p := range r.Passages {
			if p.Source != c.sources[i] || p.Kind != c.kinds[i] || p.Primary != c.primary[i] ||
				(c.labels != nil && p.Label != c.labels[i]) {
				t.Errorf("%v passage %d = %+v", c.lines, i, p)
			}
		}
	}
}

func TestBuildRejectsInvalidLines(t *testing.T) {
	if _, err := Build(hexagram.Lines{5, 7, 7, 7, 7, 7}, corpus.MustLoad()); err == nil {
		t.Error("非法爻值应报错")
	}
}

func TestEveryCastHasText(t *testing.T) {
	ic, r := corpus.MustLoad(), rng.New(1)
	for i := 0; i < 300; i++ {
		res, err := Build(CastHexagram(r), ic)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range res.Passages {
			if p.Text == "" || p.Xiang == "" {
				t.Fatalf("空经文: %+v", p)
			}
		}
	}
}

func TestObservationRelation(t *testing.T) {
	l := hexagram.Lines{9, 7, 6, 8, 7, 7} // 第 1、3 爻变
	ben := hexagram.CodeOf(l)
	flip := func(ps ...int) string {
		b := []byte(ben)
		for _, p := range ps {
			b[p-1] ^= 1
		}
		return string(b)
	}
	check := func(code, sub string) {
		t.Helper()
		if got := ObservationRelation(code, l); !contains(got, sub) {
			t.Errorf("%s: %q 应含 %q", code, got, sub)
		}
	}
	check(ben, "与本卦一致")
	check(flip(1, 3), "与变卦一致")
	check(flip(1), "中间态")
	check(flip(2), "涨落态")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPassagesCarryVernacularWhenAvailable(t *testing.T) {
	ic := corpus.MustLoad()
	check := func(lines hexagram.Lines, wantLabel string, wantVernacular bool) {
		t.Helper()
		r, err := Build(lines, ic)
		if err != nil || len(r.Passages) == 0 {
			t.Fatalf("%v: %v", lines, err)
		}
		p := r.Passages[len(r.Passages)-1]
		if p.Label != wantLabel || (p.Vernacular != "") != wantVernacular {
			t.Errorf("%v: label=%q vernacular=%q，期望 %q / 有白话=%v", lines, p.Label, p.Vernacular, wantLabel, wantVernacular)
		}
	}
	check(hexagram.Lines{9, 7, 7, 7, 7, 7}, "初九", true) // 乾初九：已有白话
	check(hexagram.Lines{9, 9, 9, 9, 9, 9}, "用九", true) // 乾用九
	check(hexagram.Lines{6, 6, 6, 6, 6, 6}, "用六", true) // 坤用六
}

func TestEveryPassageCarriesVernacular(t *testing.T) {
	// 白话已覆盖全部卦辞（64 条）与爻辞（386 条）：随机起 500 卦，读到的每一段经文都必须带白话
	ic, r := corpus.MustLoad(), rng.New(7)
	for i := 0; i < 500; i++ {
		res, err := Build(CastHexagram(r), ic)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range res.Passages {
			if p.Vernacular == "" {
				t.Fatalf("%s·%s（%s）缺少白话: %+v", p.Source, p.Label, p.Kind, p)
			}
		}
	}
}
