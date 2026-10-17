// Package reading 实现三枚铜钱起卦与断卦规则（朱熹《易学启蒙》通行读法）。
package reading

import (
	"fmt"

	"divinationmaster/internal/corpus"
	"divinationmaster/internal/hexagram"
	"divinationmaster/internal/rng"
)

// CastLine 三枚铜钱：正面（阳）记 3、反面（阴）记 2，和为 6/7/8/9，概率 1:3:3:1。
func CastLine(r *rng.Rng) int {
	sum := 0
	for i := 0; i < 3; i++ {
		sum += 2 + r.Coin()
	}
	return sum
}

// CastHexagram 自初爻起依次抛出六爻。
func CastHexagram(r *rng.Rng) hexagram.Lines {
	var l hexagram.Lines
	for i := range l {
		l[i] = CastLine(r)
	}
	return l
}

// Passage 是应读的一段经文。
type Passage struct {
	Source  string `json:"source"`
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Text    string `json:"text"`
	Xiang   string `json:"xiang"`
	Primary bool   `json:"primary"`
}

// Reading 是断卦结果。
type Reading struct {
	Rule      string    `json:"rule"`
	NChanging int       `json:"n_changing"`
	Passages  []Passage `json:"passages"`
}

// Build 按变爻数量选取应读的经文。
//
//	0 变：本卦卦辞            1 变：本卦变爻爻辞
//	2 变：本卦两变爻爻辞，以上爻为主
//	3 变：本卦卦辞 + 变卦卦辞，以本卦为主
//	4 变：变卦两个不变爻的爻辞，以下爻为主
//	5 变：变卦唯一不变爻的爻辞
//	6 变：乾坤用九/用六，其余读变卦卦辞
func Build(lines hexagram.Lines, ic *corpus.IChing) (Reading, error) {
	if err := lines.Validate(); err != nil {
		return Reading{}, err
	}
	benCode := hexagram.CodeOf(lines)
	ben, err := ic.Get(benCode)
	if err != nil {
		return Reading{}, err
	}
	changing := hexagram.ChangingPositions(lines)
	n := len(changing)
	var bian *corpus.Hexagram
	if n > 0 {
		if bian, err = ic.Get(hexagram.ChangedCodeOf(lines)); err != nil {
			return Reading{}, err
		}
	}

	judgement := func(src string, h *corpus.Hexagram, primary bool) Passage {
		return Passage{src, "卦辞", h.Name, h.Judgement, h.XiangImage, primary}
	}
	yao := func(src string, h *corpus.Hexagram, pos int, primary bool) Passage {
		ln := h.Lines[pos-1]
		return Passage{src, "爻辞", ln.Name, ln.Text, ln.Xiang, primary}
	}
	var unchanged []int
	for p := 1; p <= hexagram.NLines; p++ {
		is := false
		for _, c := range changing {
			if c == p {
				is = true
			}
		}
		if !is {
			unchanged = append(unchanged, p)
		}
	}

	var rule string
	var passages []Passage
	switch n {
	case 0:
		rule = "无变爻：以本卦卦辞为断"
		passages = []Passage{judgement("本卦", ben, true)}
	case 1:
		rule = "一爻变：以本卦变爻的爻辞为断"
		passages = []Passage{yao("本卦", ben, changing[0], true)}
	case 2:
		rule = "二爻变：读本卦两个变爻的爻辞，以上爻为主"
		passages = []Passage{yao("本卦", ben, changing[0], false), yao("本卦", ben, changing[1], true)}
	case 3:
		rule = "三爻变：本卦卦辞与变卦卦辞合参，以本卦为主"
		passages = []Passage{judgement("本卦", ben, true), judgement("变卦", bian, false)}
	case 4:
		rule = "四爻变：读变卦中两个不变爻的爻辞，以下爻为主"
		passages = []Passage{yao("变卦", bian, unchanged[0], true), yao("变卦", bian, unchanged[1], false)}
	case 5:
		rule = "五爻变：以变卦中唯一不变爻的爻辞为断"
		passages = []Passage{yao("变卦", bian, unchanged[0], true)}
	default:
		if ben.Extra != nil && (benCode == "111111" || benCode == "000000") {
			e := ben.Extra
			rule = fmt.Sprintf("六爻皆变：乾坤两卦读「%s」", e.Name)
			passages = []Passage{{"本卦", "爻辞", e.Name, e.Text, e.Xiang, true}}
		} else {
			rule = "六爻皆变：以变卦卦辞为断"
			passages = []Passage{judgement("变卦", bian, true)}
		}
	}
	return Reading{Rule: rule, NChanging: n, Passages: passages}, nil
}

// ObservationRelation 描述测量得到的卦与"本卦 → 变卦"路径的关系。
func ObservationRelation(measured string, lines hexagram.Lines) string {
	ben := hexagram.CodeOf(lines)
	changing := map[int]bool{}
	for _, p := range hexagram.ChangingPositions(lines) {
		changing[p] = true
	}
	diff := map[int]bool{}
	for i := 0; i < hexagram.NLines; i++ {
		if measured[i] != ben[i] {
			diff[i+1] = true
		}
	}
	subset, equal := true, len(diff) == len(changing)
	for p := range diff {
		if !changing[p] {
			subset, equal = false, false
		}
	}
	switch {
	case len(diff) == 0:
		return "与本卦一致"
	case len(changing) > 0 && equal:
		return "与变卦一致"
	case subset:
		return fmt.Sprintf("本卦→变卦路径上的中间态（已变 %d/%d 个变爻）", len(diff), len(changing))
	default:
		return fmt.Sprintf("涨落态（偏离本卦 %d 爻，含非变爻）", len(diff))
	}
}
