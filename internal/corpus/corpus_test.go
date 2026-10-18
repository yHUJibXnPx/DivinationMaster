package corpus

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"divinationmaster/internal/hexagram"
)

var trigram = map[string]string{"111": "乾", "000": "坤", "100": "震", "011": "巽", "010": "坎", "101": "离", "001": "艮", "110": "兑"}

func TestCounts(t *testing.T) {
	ic := MustLoad()
	n, extras := 0, map[string]string{}
	for _, h := range ic.Hexagrams {
		n += len(h.Lines)
		if h.Extra != nil {
			n++
			extras[h.Name] = h.Extra.Name
		}
	}
	if n != 386 {
		t.Errorf("爻辞条数 %d，应为 386（384 + 用九 + 用六）", n)
	}
	if len(extras) != 2 || extras["乾"] != "用九" || extras["坤"] != "用六" {
		t.Errorf("用九/用六只应属于乾坤: %v", extras)
	}
}

func TestConsistencyWithCode(t *testing.T) {
	for code, h := range MustLoad().Hexagrams {
		if h.Lower != trigram[code[:3]] || h.Upper != trigram[code[3:]] {
			t.Errorf("%s: 上下卦 %s/%s 与卦码不符", code, h.Upper, h.Lower)
		}
		for i, ln := range h.Lines {
			if ln.Pos != i+1 || ln.Name != hexagram.LineName(i, int(code[i]-'0')) || ln.Text == "" || ln.Xiang == "" {
				t.Errorf("%s 第 %d 爻数据不一致: %+v", code, i+1, ln)
			}
		}
		for _, s := range []string{h.Name, h.FullName, h.Pinyin, h.Symbol, h.Judgement, h.XiangImage, h.Interpretation} {
			if s == "" {
				t.Errorf("第 %d 卦有空字段", h.ID)
			}
		}
	}
}

func TestLegacyGuanBoFixed(t *testing.T) {
	ic := MustLoad()
	if ic.Name("000011") != "观" || ic.Name("000001") != "剥" {
		t.Errorf("观/剥卦码错误: %s %s", ic.Name("000011"), ic.Name("000001"))
	}
}

func TestXuGuaPairs(t *testing.T) {
	// 序卦"非覆即变"：奇数序号的卦与下一卦互为综卦；综卦等于自身时互为错卦
	ic := MustLoad()
	for odd := 1; odd < 64; odd += 2 {
		a, _ := ic.CodeOfID(odd)
		b, _ := ic.CodeOfID(odd + 1)
		partner := hexagram.Zong(a)
		if partner == a {
			partner = hexagram.Cuo(a)
		}
		if partner != b {
			t.Errorf("第 %d、%d 卦（%s、%s）不满足非覆即变", odd, odd+1, ic.Name(a), ic.Name(b))
		}
	}
}

func TestFind(t *testing.T) {
	ic := MustLoad()
	for _, key := range []string{"屯", "3", "100010", "水雷屯", "水雷屯卦", " 屯 "} {
		if c, ok := ic.Find(key); !ok || c != "100010" {
			t.Errorf("Find(%q) = %q, %v", key, c, ok)
		}
	}
	if _, ok := ic.Find("不存在"); ok {
		t.Error("不存在的卦不应找到")
	}
	if _, ok := ic.Find("65"); ok {
		t.Error("65 不是合法序号")
	}
}

func TestInterpretationHasNoKnownSeams(t *testing.T) {
	// 旧白话在「摘要｜详解」接缝处漏标句号，如「得此卦者不可独立创业坤象征大地」
	for _, h := range MustLoad().Hexagrams {
		re := regexp.MustCompile(`得此卦者[^，。；：！？\n]{1,24}` + regexp.QuoteMeta(h.Name) + `(?:象征|是|的本意|的意思|代表|卦)`)
		if m := re.FindString(h.Interpretation); m != "" {
			t.Errorf("第 %d 卦 %s 仍有漏标点接缝: %q", h.ID, h.Name, m)
		}
	}
}

func TestKnownSourceTyposStayFixed(t *testing.T) {
	// 源数据曾把乾九二写成「见龙再田」（爻辞与象传都错），通行本是「见龙在田」
	h, _ := MustLoad().Get("111111")
	if h.Lines[1].Text != "见龙在田，利见大人。" || h.Lines[1].Xiang != "“见龙在田”，德施普也。" {
		t.Errorf("乾九二应为「见龙在田」: %q / %q", h.Lines[1].Text, h.Lines[1].Xiang)
	}
	// 全语料里「再」只应出现在蒙卦卦辞的「再三渎」
	for _, h := range MustLoad().Hexagrams {
		fields := []string{h.Judgement, h.XiangImage}
		for _, ln := range h.Lines {
			fields = append(fields, ln.Text, ln.Xiang)
		}
		for _, f := range fields {
			if strings.Contains(f, "再") && !strings.Contains(f, "再三") {
				t.Errorf("第 %d 卦 %s 出现可疑的「再」: %s", h.ID, h.Name, f)
			}
		}
	}
}

func TestVernacularCoverageAndShape(t *testing.T) {
	ic := MustLoad()
	hexes, lines, total := ic.Coverage()
	if total != 386 {
		t.Errorf("爻总数 %d，应为 386", total)
	}
	if hexes < 28 || lines < 170 { // 进度只增不减；每完成一批就上调这里
		t.Errorf("白话覆盖只减不增：%d 卦 / %d 条", hexes, lines)
	}
	t.Logf("白话进度：%d/64 卦完整，%d/%d 条", hexes, lines, total)
	for _, h := range ic.Hexagrams {
		for _, ln := range h.Lines {
			if ln.Vernacular != "" && ln.Vernacular == ln.Text {
				t.Errorf("%s%s 的白话与原文相同", h.Name, ln.Name)
			}
		}
	}
	// 具体抽查：乾坤的用九/用六都有白话
	for code, name := range map[string]string{"111111": "用九", "000000": "用六"} {
		h, _ := ic.Get(code)
		if h.Extra == nil || h.Extra.Name != name || h.Extra.Vernacular == "" {
			t.Errorf("%s 缺少 %s 的白话", h.Name, name)
		}
	}
}

func TestMalformedVernacularIsRejected(t *testing.T) {
	good := vernacularRaw
	defer func() { vernacularRaw = good }()
	six := `["一。","二。","三。","四。","五。","六。"]`
	cases := map[string]string{
		"未知卦码":       `{"hexagrams":{"1111111":{"lines":` + six + `,"extra":"甲。"}}}`,
		"只有五条":       `{"hexagrams":{"111111":{"lines":["一。","二。","三。","四。","五。"],"extra":"甲。"}}}`,
		"缺句号":        `{"hexagrams":{"111111":{"lines":["一。","二","三。","四。","五。","六。"],"extra":"甲。"}}}`,
		"空字符串":       `{"hexagrams":{"111111":{"lines":["一。","","三。","四。","五。","六。"],"extra":"甲。"}}}`,
		"乾缺用九":       `{"hexagrams":{"111111":{"lines":` + six + `}}}`,
		"屯不该有 extra": `{"hexagrams":{"100010":{"lines":` + six + `,"extra":"甲。"}}}`,
		"extra 缺句号":  `{"hexagrams":{"111111":{"lines":` + six + `,"extra":"甲"}}}`,
		"不是 JSON":    `{坏掉`,
	}
	for name, doc := range cases {
		vernacularRaw = []byte(doc)
		if _, err := Load(); err == nil {
			t.Errorf("%s：应被拒绝", name)
		}
	}
	vernacularRaw = []byte(`{"hexagrams":{"100010":{"lines":` + six + `}}}`)
	if _, err := Load(); err != nil {
		t.Errorf("合法数据被拒绝: %v", err)
	}
}

func TestXiangTyposStayFixed(t *testing.T) {
	// 象传里三处曾与爻辞/通行本不符的笔误
	ic := MustLoad()
	check := func(hid int, line int, want string) {
		t.Helper()
		code, _ := ic.CodeOfID(hid)
		h, _ := ic.Get(code)
		if got := h.Lines[line].Xiang; !strings.Contains(got, want) {
			t.Errorf("第 %d 卦 %s %s 象传应含 %q，实为 %q", hid, h.Name, h.Lines[line].Name, want, got)
		}
	}
	check(6, 0, "虽“小有言”")  // 讼初六
	check(7, 4, "“弟子舆尸”")  // 师六五
	check(35, 0, "“晋如摧如”") // 晋初六
}

func TestXiangQuotesMatchLineText(t *testing.T) {
	// 象传里引用爻辞的片段，应是爻辞原文的子串。已知的例外都是象传本身的节引/概括/标题。
	known := map[string]bool{
		"1乾用九": true, "10履六三": true, "18蛊九三": true, "33遁六二": true, "33遁九三": true, "46升上六": true,
	}
	quote := regexp.MustCompile(`“([^”]+)”`)
	strip := strings.NewReplacer("，", "", "。", "", "；", "", "：", "", "！", "", "？", "", "、", "", "“", "", "”", "")
	for _, h := range MustLoad().Hexagrams {
		type item struct{ name, text, xiang string }
		items := []item{}
		for _, ln := range h.Lines {
			items = append(items, item{ln.Name, ln.Text, ln.Xiang})
		}
		if h.Extra != nil {
			items = append(items, item{h.Extra.Name, h.Extra.Text, h.Extra.Xiang})
		}
		for _, it := range items {
			for _, m := range quote.FindAllStringSubmatch(it.xiang, -1) {
				if !strings.Contains(strip.Replace(it.text), strip.Replace(m[1])) && !known[fmt.Sprintf("%d%s%s", h.ID, h.Name, it.name)] {
					t.Errorf("第 %d 卦 %s %s：象传引「%s」不在爻辞里（爻辞：%s）", h.ID, h.Name, it.name, m[1], it.text)
				}
			}
		}
	}
}

func TestVernacularConvention(t *testing.T) {
	ic := MustLoad()
	// 凡与字面古义有出入的转写，必须用“（原文……”明示
	mustDisclose := map[string][]int{
		"100010": {0, 1},       // 屯：利建侯、女子待字
		"010001": {1, 2},       // 蒙：纳妇、取女见金夫
		"010111": {2},          // 讼：王事
		"010000": {0, 2, 4},    // 师：出兵、舆尸、长子弟子
		"111011": {5},          // 小畜：妇贞厉
		"110111": {2},          // 履：眇能视、跛能履（以缺陷作比喻）
		"111000": {4, 5},       // 泰：帝乙归妹、勿用师
		"000111": {1},          // 否：小人/大人
		"101111": {1, 2, 3, 4}, // 同人：宗、伏戎、乘墉、大师克
		"111101": {2},          // 大有：公用亨于天子
		"001000": {4, 5},       // 谦：侵伐、行师征邑国
		"100110": {1, 2},       // 随：小子/丈夫
		"011001": {0, 1, 5},    // 蛊：干父、干母、不事王侯
		"110000": {4},          // 临：大君之宜
		"000011": {0, 1, 3},    // 观：小人君子、利女贞、宾于王
		"100101": {0, 5},       // 噬嗑：屦校灭趾、何校灭耳（刑具）
		"101001": {3},          // 贲：婚媾
		"000001": {4, 5},       // 剥：宫人宠、小人剥庐
		"100000": {5},          // 复：用行师
		"111001": {4},          // 大畜：豮豕
		"011110": {1, 4},       // 大过：老夫女妻、老妇士夫
	}
	for code, idxs := range mustDisclose {
		h, _ := ic.Get(code)
		for _, i := range idxs {
			if !strings.Contains(h.Lines[i].Vernacular, "（原文") {
				t.Errorf("%s%s 属于转写，白话必须含“（原文……”说明: %q", h.Name, h.Lines[i].Name, h.Lines[i].Vernacular)
			}
		}
	}
	// 所有白话的中文括号必须成对
	for _, h := range ic.Hexagrams {
		all := []string{}
		for _, ln := range h.Lines {
			all = append(all, ln.Vernacular)
		}
		if h.Extra != nil {
			all = append(all, h.Extra.Vernacular)
		}
		for _, v := range all {
			if strings.Count(v, "（") != strings.Count(v, "）") {
				t.Errorf("%s 的白话括号不成对: %q", h.Name, v)
			}
		}
	}
}
