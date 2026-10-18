// Package golden 是冻结的回归快照测试：把 RNG 序列、引擎概率分布、读法、整次占卜、
// CLI 整段输出固定在 testdata/golden.json 里，任何无意的改动都会被抓到。
//
// 来历：最初的向量由已移除的 Python 参考实现生成，并逐项验证与 Go 实现一致
// （RNG 位级一致；概率分布 1e-12 内一致；读法与终端输出逐字一致）。
//
// 有意改动引擎或渲染之后，用下面的命令重新生成，并在 git diff 里逐项审阅变化：
//
//	go test ./internal/golden -update
//
// 注意：重新生成之后，快照就是 Go 自己产出的，不再是独立的参照物——所以必须审阅 diff，
// 不能盲目更新。
package golden

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
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

var update = flag.Bool("update", false, "重新生成 testdata/golden.json（仅在有意改动引擎/渲染后使用，并审阅 diff）")

const (
	root       = "../../testdata/"
	goldenFile = root + "golden.json"
	histSample = root + "history_sample.jsonl"
	note       = "冻结的回归快照；初版由已移除的 Python 参考实现生成并验证。用 go test ./internal/golden -update 重新生成并审阅 diff，勿手改。"
	tolerance  = 1e-9
)

type cfgJSON struct {
	Fuzz      float64 `json:"fuzz"`
	Entangle  float64 `json:"entangle"`
	Interfere float64 `json:"interfere"`
}

func (c cfgJSON) cfg() quantum.Config {
	return quantum.Config{Fuzz: c.Fuzz, Entangle: c.Entangle, Interfere: c.Interfere}
}

type rngCase struct {
	Seed   string   `json:"seed"`
	U64    []string `json:"u64"`
	Floats []string `json:"floats"`
	Coins  []int    `json:"coins"`
}

type engineCase struct {
	Lines            []int     `json:"lines"`
	Question         string    `json:"question"`
	Seed             string    `json:"seed"`
	Config           cfgJSON   `json:"config"`
	Probabilities    []float64 `json:"probabilities"`
	Entropy          float64   `json:"entropy"`
	LineEntanglement []float64 `json:"line_entanglement"`
	Top              [][]any   `json:"top"`
	Measured         string    `json:"measured"`
}

type readingCase struct {
	Lines   []int           `json:"lines"`
	Reading reading.Reading `json:"reading"`
}

type divineCase struct {
	Question string           `json:"question"`
	Seed     uint64           `json:"seed"`
	Config   cfgJSON          `json:"config"`
	Lines    []int            `json:"lines"`
	Changing []int            `json:"changing"`
	Benguo   core.Ref         `json:"benguo"`
	Biangua  *core.Ref        `json:"biangua"`
	Derived  core.Derived     `json:"derived"`
	Reading  reading.Reading  `json:"reading"`
	Quantum  core.QuantumView `json:"quantum"`
}

type cliCase struct {
	Args   []string `json:"args"`
	Exit   int      `json:"exit"`
	Stdout string   `json:"stdout"`
}

type golden struct {
	Note        string        `json:"note"`
	RNG         []rngCase     `json:"rng"`
	Engine      []engineCase  `json:"engine"`
	Reading     []readingCase `json:"reading"`
	Divine      []divineCase  `json:"divine"`
	CLI         []cliCase     `json:"cli"`
	HistoryRows []string      `json:"history_rows"`
}

// ---- 输入集合（前半沿用最初 Python 生成器的输入，保证旧用例不变；后半是新模型用例） ----

var (
	oldDefault = cfgJSON{0.2, math.Pi / 2, 0.35} // 旧版默认值（fuzz=0.2），显式写出
	newDefault = cfgJSON{0, math.Pi / 2, 0.35}   // 当前默认值

	engineLines = [][]int{
		{7, 8, 7, 8, 7, 8}, {9, 7, 7, 6, 7, 7}, {9, 6, 9, 6, 9, 6}, {9, 7, 6, 8, 9, 7},
		{9, 9, 9, 9, 9, 9}, {6, 6, 6, 6, 6, 6}, {8, 8, 8, 8, 8, 8}, {9, 8, 7, 6, 8, 7}, {6, 9, 7, 7, 8, 8},
	}
	engineCfgs = []cfgJSON{oldDefault, {0, 0, 0}, {0, math.Pi / 2, 0}, {0.5, 1.0, 0.9}}
	// 新模型：不变的爻绝对稳定，干涉只作用于变爻
	engineCfgsNew = []cfgJSON{newDefault, {0, 1.0, 0.8}}

	readingLines = [][]int{
		{7, 8, 7, 8, 7, 8}, {9, 7, 7, 7, 7, 7}, {9, 7, 6, 7, 7, 7}, {9, 9, 9, 7, 7, 7},
		{9, 9, 9, 9, 7, 7}, {9, 9, 9, 9, 9, 7}, {9, 9, 9, 9, 9, 9}, {6, 6, 6, 6, 6, 6}, {9, 6, 6, 6, 9, 6},
		{6, 8, 7, 9, 6, 8}, {7, 7, 6, 6, 6, 6}, {8, 9, 8, 9, 8, 9},
	}

	divineCfgsTail = []cfgJSON{
		{0, 0, 0}, {0.4, 2.0, 0.8}, {0.1, math.Pi, 0.2}, {0, math.Pi / 2, 0}, {0.3, 0, 0.5}, {0.2, 0.7, 0},
	}

	cliArgs = [][]string{
		{"我该接受这份工作吗？", "--seed", "7", "--no-save"},
		{"ask", "问题乙", "--seed", "12345", "--top", "3", "--no-save"},
		{"--seed", "9", "--fuzz", "0", "--entangle", "0", "--interfere", "0", "--no-save", "零参数"},
		{"ask", "", "--seed", "0", "--no-save"},
		{"ask", "大数种子", "--seed", "18446744073709551615", "--top", "8", "--no-save"},
		{"show", "屯"}, {"show", "乾"}, {"show", "坤"}, {"show", "20"}, {"show", "100010"}, {"show", "水雷屯"},
		{"show", "不存在的卦"}, {"ask", "q", "--fuzz", "5", "--no-save"},
		// 新增：当前默认模型的三种典型情形，以及与旧默认值（fuzz=0.2）的对照
		{"ask", "有纠缠（初↔四同为变爻）", "--seed", "8", "--no-save"},
		{"ask", "多变爻且有纠缠", "--seed", "11", "--no-save"},
		{"ask", "静卦（无变爻）", "--seed", "25", "--no-save"},
		{"ask", "旧默认值对照", "--seed", "8", "--fuzz", "0.2", "--no-save"},
	}
)

// ---- 由输入计算输出 ----

func mustLines(v []int) hexagram.Lines {
	var l hexagram.Lines
	if len(v) != hexagram.NLines {
		panic(fmt.Sprintf("lines 长度 %d", len(v)))
	}
	copy(l[:], v)
	return l
}

func computeRNG(seed uint64) rngCase {
	c := rngCase{Seed: strconv.FormatUint(seed, 10)}
	r := rng.New(seed)
	for i := 0; i < 8; i++ {
		c.U64 = append(c.U64, strconv.FormatUint(r.Uint64(), 10))
	}
	r = rng.New(seed)
	for i := 0; i < 4; i++ {
		c.Floats = append(c.Floats, strconv.FormatFloat(r.Float64(), 'g', -1, 64))
	}
	r = rng.New(seed)
	for i := 0; i < 16; i++ {
		c.Coins = append(c.Coins, r.Coin())
	}
	return c
}

func computeEngine(lines []int, question string, seed uint64, c cfgJSON) engineCase {
	st := quantum.Run(mustLines(lines), question, seed, c.cfg())
	probs := st.Probabilities()
	e := engineCase{
		Lines: lines, Question: question, Seed: strconv.FormatUint(seed, 10), Config: c,
		Probabilities: probs[:], Entropy: st.Entropy(),
		Measured: hexagram.IndexToCode(st.Measure(rng.New(seed))),
	}
	for q := 0; q < hexagram.NLines; q++ {
		e.LineEntanglement = append(e.LineEntanglement, st.LineEntanglement(q))
	}
	for _, t := range st.Top(5) {
		e.Top = append(e.Top, []any{hexagram.IndexToCode(t.Index), t.P})
	}
	return e
}

func computeDivine(ic *corpus.IChing, question string, seed uint64, c cfgJSON) divineCase {
	r, err := core.Divine(question, seed, c.cfg(), ic, 5)
	if err != nil {
		panic(err)
	}
	return divineCase{question, seed, c, r.Lines, r.Changing, r.Benguo, r.Biangua, r.Derived, r.Reading, r.Quantum}
}

func computeCLI(args []string) cliCase {
	var out, errb bytes.Buffer
	code := cli.Main(args, bytes.NewReader(nil), &out, &errb)
	return cliCase{Args: args, Exit: code, Stdout: out.String()}
}

func build() *golden {
	ic := corpus.MustLoad()
	g := &golden{Note: note}
	for _, seed := range []uint64{0, 1, 42, 12345678901234567890, math.MaxUint64} {
		g.RNG = append(g.RNG, computeRNG(seed))
	}
	n := 0
	for _, cfgs := range [][]cfgJSON{engineCfgs, engineCfgsNew} {
		for _, lines := range engineLines {
			for _, c := range cfgs {
				n++
				g.Engine = append(g.Engine, computeEngine(lines, fmt.Sprintf("引擎对拍%d", n), uint64(1000+n), c))
			}
		}
	}
	for _, lines := range readingLines {
		r, err := reading.Build(mustLines(lines), ic)
		if err != nil {
			panic(err)
		}
		g.Reading = append(g.Reading, readingCase{lines, r})
	}
	for i := 0; i < 36; i++ {
		c := oldDefault
		switch {
		case i >= 30:
			c = newDefault
		case i >= 24:
			c = divineCfgsTail[i-24]
		}
		question := fmt.Sprintf("第%d个问题：我该怎么做？", i)
		if i == 0 {
			question = ""
		}
		seed := uint64(i*7919 + 3)
		if i >= 28 && i < 30 {
			seed = math.MaxUint64 - uint64(i)
		}
		g.Divine = append(g.Divine, computeDivine(ic, question, seed, c))
	}
	for _, args := range cliArgs {
		g.CLI = append(g.CLI, computeCLI(args))
	}
	recs, err := history.Load(histSample, os.Stderr)
	if err != nil {
		panic(err)
	}
	for _, r := range recs {
		g.HistoryRows = append(g.HistoryRows, render.HistoryRow(r))
	}
	return g
}

// ---- 比较与读写 ----

func tree(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

// diff 递归比较两棵 JSON 树：数字允许 tolerance 的误差，其余必须完全相等。
func diff(path string, want, got any, out *[]string) {
	if len(*out) >= 5 {
		return
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			*out = append(*out, path+": 对象结构不同")
			return
		}
		for k, wv := range w {
			gv, present := g[k]
			if !present {
				*out = append(*out, path+"."+k+": 缺失")
				continue
			}
			diff(path+"."+k, wv, gv, out)
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			*out = append(*out, fmt.Sprintf("%s: 数组长度不同", path))
			return
		}
		for i := range w {
			diff(fmt.Sprintf("%s[%d]", path, i), w[i], g[i], out)
		}
	case float64:
		if g, ok := got.(float64); !ok || math.Abs(g-w) > tolerance {
			*out = append(*out, fmt.Sprintf("%s: want %v, got %v", path, want, got))
		}
	case string:
		if g, _ := got.(string); g != w {
			*out = append(*out, fmt.Sprintf("%s: %s", path, firstDifference(w, g)))
		}
	default:
		if want != got {
			*out = append(*out, fmt.Sprintf("%s: want %v, got %v", path, want, got))
		}
	}
}

// firstDifference 指出两段文本第一处不同的行（整段输出很长，全部倒出来没法看）。
func firstDifference(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("第 %d 行不同\n   want %q\n   got  %q", i+1, w, g)
		}
	}
	return "内容不同"
}

func writeGolden(g *golden) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(g); err != nil {
		return err
	}
	return os.WriteFile(goldenFile, buf.Bytes(), 0o644)
}

func TestGolden(t *testing.T) {
	got := build()
	if *update {
		if err := writeGolden(got); err != nil {
			t.Fatal(err)
		}
		t.Logf("已重新生成 %s：请用 git diff 逐项审阅变化", goldenFile)
		return
	}
	raw, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", goldenFile, err)
	}
	var want map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	gotTree := tree(got).(map[string]any)
	for _, section := range []string{"rng", "engine", "reading", "divine", "cli", "history_rows"} {
		t.Run(section, func(t *testing.T) {
			var diffs []string
			diff(section, want[section], gotTree[section], &diffs)
			for _, d := range diffs {
				t.Error(d)
			}
		})
	}
}

// 历史记录格式向后兼容：旧版写出的 history.jsonl（含已不再显示的字段）仍可读取并渲染。
func TestOldHistoryFormatStillReadable(t *testing.T) {
	recs, err := history.Load(histSample, os.Stderr)
	if err != nil || len(recs) == 0 {
		t.Fatalf("读取样例失败: %v (n=%d)", err, len(recs))
	}
	for _, r := range recs {
		if row := render.HistoryRow(r); row == "" || r.Benguo.Name == "" || r.Quantum.Measured.Name == "" {
			t.Errorf("记录不完整: %+v", r)
		}
	}
}
