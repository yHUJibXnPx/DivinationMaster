// Package parity 用 Python 参考实现生成的 testdata/golden.json 对拍 Go 版。
// 重新生成：在项目根目录运行 python tools/gen_golden.py。
package parity

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strconv"
	"testing"

	"divinationmaster/internal/cli"
	"divinationmaster/internal/core"
	"divinationmaster/internal/corpus"
	"divinationmaster/internal/hexagram"
	"divinationmaster/internal/history"
	"divinationmaster/internal/quantum"
	"divinationmaster/internal/reading"
	"divinationmaster/internal/render"
	"divinationmaster/internal/rng"
)

const root = "../../../testdata/"

type cfgJSON struct{ Fuzz, Entangle, Interfere float64 }

type golden struct {
	RNG []struct {
		Seed   string
		U64    []string
		Floats []string
		Coins  []int
	}
	Engine []struct {
		Lines             []int
		Question          string
		Seed              string
		Config            cfgJSON
		Probabilities     []float64
		Entropy           float64
		Line_entanglement []float64
		Top               [][]any
		Measured          string
	}
	Reading []struct {
		Lines   []int
		Reading reading.Reading
	}
	Divine []core.Result
	CLI    []struct {
		Args   []string
		Exit   int
		Stdout string
	}
	History_rows []string
}

func load(t *testing.T) *golden {
	t.Helper()
	b, err := os.ReadFile(root + "golden.json")
	if err != nil {
		t.Fatalf("读取 golden.json 失败（先运行 python tools/gen_golden.py）: %v", err)
	}
	var g golden
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	return &g
}

func toLines(t *testing.T, v []int) hexagram.Lines {
	t.Helper()
	var l hexagram.Lines
	if len(v) != hexagram.NLines {
		t.Fatalf("lines 长度 %d", len(v))
	}
	copy(l[:], v)
	return l
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestRNG(t *testing.T) {
	for _, c := range load(t).RNG {
		seed, err := strconv.ParseUint(c.Seed, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		r := rng.New(seed)
		for i, want := range c.U64 {
			if got := strconv.FormatUint(r.Uint64(), 10); got != want {
				t.Fatalf("seed %s u64[%d] = %s, want %s", c.Seed, i, got, want)
			}
		}
		r = rng.New(seed)
		for i, want := range c.Floats {
			w, _ := strconv.ParseFloat(want, 64)
			if got := r.Float64(); got != w { // 位级相等
				t.Fatalf("seed %s float[%d] = %v, want %v", c.Seed, i, got, w)
			}
		}
		r = rng.New(seed)
		for i, want := range c.Coins {
			if got := r.Coin(); got != want {
				t.Fatalf("seed %s coin[%d] = %d, want %d", c.Seed, i, got, want)
			}
		}
	}
}

func TestEngine(t *testing.T) {
	for n, c := range load(t).Engine {
		seed, _ := strconv.ParseUint(c.Seed, 10, 64)
		cfg := quantum.Config{Fuzz: c.Config.Fuzz, Entangle: c.Config.Entangle, Interfere: c.Config.Interfere}
		st := quantum.Run(toLines(t, c.Lines), c.Question, seed, cfg)
		probs := st.Probabilities()
		for i, w := range c.Probabilities {
			if !near(probs[i], w, 1e-12) {
				t.Fatalf("case %d p[%d] = %.17g, want %.17g", n, i, probs[i], w)
			}
		}
		if !near(st.Entropy(), c.Entropy, 1e-10) {
			t.Fatalf("case %d entropy %v vs %v", n, st.Entropy(), c.Entropy)
		}
		for q, w := range c.Line_entanglement {
			if !near(st.LineEntanglement(q), w, 1e-9) {
				t.Fatalf("case %d entanglement[%d] %v vs %v", n, q, st.LineEntanglement(q), w)
			}
		}
		for i, tp := range st.Top(len(c.Top)) {
			if code := hexagram.IndexToCode(tp.Index); code != c.Top[i][0].(string) {
				t.Fatalf("case %d top[%d] = %s, want %s", n, i, code, c.Top[i][0])
			}
			if !near(tp.P, c.Top[i][1].(float64), 1e-12) {
				t.Fatalf("case %d top[%d] p mismatch", n, i)
			}
		}
		if got := hexagram.IndexToCode(st.Measure(rng.New(seed))); got != c.Measured {
			t.Fatalf("case %d measured %s, want %s", n, got, c.Measured)
		}
	}
}

func TestReading(t *testing.T) {
	ic := corpus.MustLoad()
	for n, c := range load(t).Reading {
		got, err := reading.Build(toLines(t, c.Lines), ic)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, c.Reading) {
			t.Fatalf("case %d (%v):\n got  %+v\n want %+v", n, c.Lines, got, c.Reading)
		}
	}
}

func TestDivine(t *testing.T) {
	ic := corpus.MustLoad()
	for n, want := range load(t).Divine {
		cfg := quantum.Config{Fuzz: want.Config.Fuzz, Entangle: want.Config.Entangle, Interfere: want.Config.Interfere}
		got, err := core.Divine(want.Question, want.Seed, cfg, ic, 5)
		if err != nil {
			t.Fatal(err)
		}
		// 精确比较离散部分
		if !reflect.DeepEqual(got.Lines, want.Lines) || !reflect.DeepEqual(got.Changing, want.Changing) ||
			!reflect.DeepEqual(got.Benguo, want.Benguo) || !reflect.DeepEqual(got.Biangua, want.Biangua) ||
			!reflect.DeepEqual(got.Derived, want.Derived) || !reflect.DeepEqual(got.Reading, want.Reading) ||
			got.Quantum.Measured != want.Quantum.Measured || got.Quantum.Relation != want.Quantum.Relation ||
			got.Seed != want.Seed || got.Question != want.Question || got.Config != want.Config {
			t.Fatalf("case %d 离散字段不一致:\n got  %+v\n want %+v", n, got, want)
		}
		// 容差比较连续部分
		if !near(got.Quantum.EntropyBits, want.Quantum.EntropyBits, 1e-10) {
			t.Fatalf("case %d entropy", n)
		}
		for i := range want.Quantum.LineEntanglement {
			if !near(got.Quantum.LineEntanglement[i], want.Quantum.LineEntanglement[i], 1e-9) {
				t.Fatalf("case %d entanglement[%d]", n, i)
			}
		}
		if len(got.Quantum.Top) != len(want.Quantum.Top) {
			t.Fatalf("case %d top 长度", n)
		}
		for i, w := range want.Quantum.Top {
			g := got.Quantum.Top[i]
			if g.Code != w.Code || g.ID != w.ID || g.Name != w.Name || !near(g.P, w.P, 1e-12) {
				t.Fatalf("case %d top[%d]: got %+v want %+v", n, i, g, w)
			}
		}
	}
}

func TestCLIOutputIdentical(t *testing.T) {
	for _, c := range load(t).CLI {
		var out, errb bytes.Buffer
		code := cli.Main(c.Args, bytes.NewReader(nil), &out, &errb)
		if code != c.Exit {
			t.Errorf("%v: exit %d, want %d (stderr=%q)", c.Args, code, c.Exit, errb.String())
			continue
		}
		if out.String() != c.Stdout {
			t.Errorf("%v: stdout 与 Python 版不一致\n--- got ---\n%s\n--- want ---\n%s", c.Args, out.String(), c.Stdout)
		}
	}
}

func TestReadsPythonHistory(t *testing.T) {
	recs, err := history.Load(root+"history_sample.jsonl", os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := load(t).History_rows
	if len(recs) != len(want) || len(recs) == 0 {
		t.Fatalf("记录数 %d, want %d", len(recs), len(want))
	}
	for i, r := range recs {
		if got := render.HistoryRow(r); got != want[i] {
			t.Errorf("row %d: %q, want %q", i, got, want[i])
		}
	}
}
