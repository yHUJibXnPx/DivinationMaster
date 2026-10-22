package corpus

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

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
	if hexes != 64 || lines != 386 { // 白话已覆盖全部 64 卦；今后任何一条丢失都应当报错
		t.Errorf("白话应完整覆盖 64 卦 / 386 条，实为 %d 卦 / %d 条", hexes, lines)
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
		"整卦白话缺句号":    `{"hexagrams":{"100010":{"summary":"没有句号","lines":` + six + `}}}`,
		"卦辞白话缺句号":    `{"hexagrams":{"100010":{"judgement":"没有句号","lines":` + six + `}}}`,
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
	check(6, 0, "虽“小有言”") // 讼初六
	check(7, 4, "“弟子舆尸”") // 师六五
	check(35, 0, "“晋如摧如”")
	check(31, 4, "志末也") // 咸九五（形近字：末/未） // 晋初六
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
		"100010": {0, 1, 3},          // 屯：利建侯、女子待字
		"010001": {0, 1, 2},          // 蒙：纳妇、取女见金夫
		"010111": {2},                // 讼：王事
		"010000": {0, 1, 2, 3, 4, 5}, // 师：出兵、舆尸、长子弟子
		"111011": {5},                // 小畜：妇贞厉
		"110111": {2},                // 履：眇能视、跛能履（以缺陷作比喻）
		"111000": {4, 5},             // 泰：帝乙归妹、勿用师
		"000111": {1},                // 否：小人/大人
		"101111": {1, 2, 3, 4},       // 同人：宗、伏戎、乘墉、大师克
		"111101": {2},                // 大有：公用亨于天子
		"001000": {4, 5},             // 谦：侵伐、行师征邑国
		"100110": {1, 2},             // 随：小子/丈夫
		"011001": {0, 1, 5},          // 蛊：干父、干母、不事王侯
		"110000": {4},                // 临：大君之宜
		"000011": {0, 1, 3},          // 观：小人君子、利女贞、宾于王
		"100101": {0, 5},             // 噬嗑：屦校灭趾、何校灭耳（刑具）
		"101001": {3},                // 贲：婚媾
		"000001": {4, 5},             // 剥：宫人宠、小人剥庐
		"100000": {5},                // 复：用行师
		"111001": {4},                // 大畜：豮豕
		"011110": {1, 4},             // 大过：老夫女妻、老妇士夫
		"010010": {5},                // 坎：係用徽纆、寘于丛棘（牢狱）
		"101101": {5},                // 离：王用出征、折首
		"011100": {4},                // 恒：妇人/夫子
		"001111": {2, 3},             // 遁：畜臣妾、君子/小人
		"111100": {2},                // 大壮：小人用壮、君子用罔
		"000101": {1, 5},             // 晋：王母、伐邑
		"101000": {2, 4},             // 明夷：得其大首、箕子
		"101011": {1, 2, 4},          // 家人：在中馈、妇子、王假有家
		"110101": {2, 4, 5},          // 睽：天且劓、厥宗、婚媾
		"001010": {1},                // 蹇：王臣
		"010100": {1, 2, 4, 5},       // 解：田获三狐、负且乘、小人、射隼
		"110001": {4, 5},             // 损：十朋之龟、得臣无家
		"100011": {1, 2, 3},          // 益：王用享于帝、迁国
		"111110": {1},                // 夬：有戎
		"011111": {0},                // 姤：一阴遇五阳、女壮
		"000110": {1},                // 萃：禴
		"011000": {1, 2, 3},          // 升：禴、虚邑、岐山
		"010110": {1, 2, 4},          // 困：朱绂、不见其妻、劓刖
		"011010": {2},                // 井：王明
		"101110": {5},                // 革：小人革面
		"011101": {0},                // 鼎：得妾以其子
		"100100": {4, 5},             // 震：有事、婚媾
		"001001": {2},                // 艮：列其夤
		"001011": {2, 4},             // 渐：夫征不复妇孕不育、妇三岁不孕
		"110100": {0, 1, 2, 3, 4, 5}, // 归妹：整卦以嫁娶、陪嫁、婚祭仪式为喻，每一爻都必须披露
		"101100": {2},                // 丰：折其右肱
		"001101": {1, 2},             // 旅：童仆
		"011011": {1, 3},             // 巽：史巫、田获三品
		"110110": {4},                // 兑：孚于剥
		"010011": {4},                // 涣：涣王居
		"001100": {1, 4},             // 小过：祖妣君臣、公弋取彼在穴
		"101010": {1, 2, 4},          // 既济：妇丧其茀、高宗伐鬼方、杀牛禴祭
		"010101": {3},                // 未济：震用伐鬼方
		"000000": {2},                // 坤六三：王事
		"000010": {4},                // 比九五：王用三驱
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

func TestInterpretationsHaveNoDerogatoryQuotes(t *testing.T) {
	// 咸卦整卦白话曾引用「唯女子与小人难养也」（对女性的贬损），已改写；不得再出现
	for _, h := range MustLoad().Hexagrams {
		for _, bad := range []string{"唯女子与小人难养", "女子与小人"} {
			if strings.Contains(h.Interpretation, bad) {
				t.Errorf("第 %d 卦 %s 的整卦白话含有贬损性引文 %q", h.ID, h.Name, bad)
			}
		}
	}
}

func TestSummaryStyle(t *testing.T) {
	// 整卦白话（每次占卜最先读到的那段话）：新写的、有统一体例、没有断言式的算命口吻
	forbidden := []string{"得此卦者", "得卦者", "注定", "必然", "一定会", "唯女子与小人", "女子与小人"}
	seen := map[string]int{}
	for _, h := range MustLoad().Hexagrams {
		s := h.Interpretation
		if n := utf8.RuneCountInString(s); n < 60 || n > 220 {
			t.Errorf("第 %d 卦 %s 的整卦白话长度 %d，应在 60–220 字之间", h.ID, h.Name, n)
		}
		if !strings.Contains(s, h.Name) { // 防止把第 N 卦的白话错贴到第 M 卦
			t.Errorf("第 %d 卦 %s 的整卦白话没有提到卦名，疑似张冠李戴: %q", h.ID, h.Name, s)
		}
		if !strings.HasSuffix(s, "。") && !strings.HasSuffix(s, "）") {
			t.Errorf("第 %d 卦 %s 的整卦白话没有以句号收尾", h.ID, h.Name)
		}
		if strings.HasPrefix(s, "代表") {
			t.Errorf("第 %d 卦 %s 的整卦白话还是旧版格式（以“代表”开头）", h.ID, h.Name)
		}
		for _, bad := range forbidden {
			if strings.Contains(s, bad) {
				t.Errorf("第 %d 卦 %s 的整卦白话含有不宜的措辞 %q", h.ID, h.Name, bad)
			}
		}
		if prev, dup := seen[s]; dup {
			t.Errorf("第 %d 卦与第 %d 卦的整卦白话完全相同", h.ID, prev)
		}
		seen[s] = h.ID
	}
}

func TestJudgementVernacular(t *testing.T) {
	ic := MustLoad()
	// 手写的"关键词表"：每卦的卦辞白话必须出现的词（全部出现）。它与写数据的脚本相互独立，
	// 用来抓"把第 N 卦的卦辞白话错贴到第 M 卦"。注意：不能用字符重合度来做这件事——
	// 很多卦辞是「元亨利贞」之类的程式化用语，真配对的重合度可低至 0%，错配却可高达 100%。
	anchors := map[int][]string{
		1:  {"元亨利贞"},
		2:  {"母马", "西南"},
		3:  {"根基"},
		4:  {"求教"},
		5:  {"等待"},
		6:  {"戒惧"},
		7:  {"长者"},
		8:  {"来得迟"},
		9:  {"浓云"},
		10: {"老虎"},
		11: {"小的离去"},
		12: {"闭塞"},
		13: {"旷野"},
		14: {"拥有"},
		15: {"谦逊"},
		16: {"安乐"},
		17: {"顺随"},
		18: {"甲日"},
		19: {"八月"},
		20: {"洗了手"},
		21: {"处理案件"},
		22: {"修饰"},
		23: {"剥落"},
		24: {"七天"},
		25: {"不虚妄"},
		26: {"不家食"},
		27: {"口食"},
		28: {"栋梁"},
		29: {"险陷"},
		30: {"母牛"},
		31: {"感应"},
		32: {"恒心"},
		33: {"退避"},
		34: {"逞强"},
		35: {"赏赐"},
		36: {"光明受伤"},
		37: {"家庭"},
		38: {"背离"},
		39: {"西南", "拜见有见识的人"},
		40: {"回到原位"},
		41: {"两碗"},
		42: {"增益"},
		43: {"朝堂"},
		44: {"新出现"},
		45: {"宗庙", "大的祭品"},
		46: {"向南"},
		47: {"困境"},
		48: {"水井"},
		49: {"合适的日子"},
		50: {"去旧取新"},
		51: {"雷声"},
		52: {"止在背上"},
		53: {"循序渐进"},
		54: {"嫁妹"},
		55: {"正午"},
		56: {"行旅"},
		57: {"顺入"},
		58: {"喜悦"},
		59: {"宗庙", "涣散"},
		60: {"节制"},
		61: {"小猪"},
		62: {"飞鸟"},
		63: {"已经完成"},
		64: {"小狐狸"},
	}
	// 与字面古义有出入的转写，必须用“（原文……”明示
	mustDisclose := []int{2, 3, 4, 7, 8, 11, 12, 16, 19, 20, 21, 24, 30, 31, 35, 37, 39, 40, 41, 43, 44, 45, 46, 51, 53, 54, 55, 59, 61}
	disclose := map[int]bool{}
	for _, id := range mustDisclose {
		disclose[id] = true
	}
	seen := map[string]int{}
	forbidden := []string{"得此卦者", "注定", "必然", "一定会", "女子与小人"}
	for _, h := range ic.Hexagrams {
		v := h.JudgementVernacular
		if v == "" {
			t.Errorf("第 %d 卦 %s 缺少卦辞白话", h.ID, h.Name)
			continue
		}
		if !strings.HasSuffix(v, "。") && !strings.HasSuffix(v, "）") {
			t.Errorf("第 %d 卦 %s 的卦辞白话没有以句号收尾", h.ID, h.Name)
		}
		if v == h.Judgement {
			t.Errorf("第 %d 卦 %s 的卦辞白话与原文相同", h.ID, h.Name)
		}
		if strings.Count(v, "（") != strings.Count(v, "）") {
			t.Errorf("第 %d 卦 %s 的卦辞白话括号不成对", h.ID, h.Name)
		}
		for _, a := range anchors[h.ID] {
			if !strings.Contains(v, a) {
				t.Errorf("第 %d 卦 %s 的卦辞白话应含关键词 %q，疑似错位: %q", h.ID, h.Name, a, v)
			}
		}
		if disclose[h.ID] != strings.Contains(v, "（原文") {
			t.Errorf("第 %d 卦 %s：转写披露与清单不一致（清单要求=%v，实际含“（原文”=%v）", h.ID, h.Name, disclose[h.ID], strings.Contains(v, "（原文"))
		}
		for _, bad := range forbidden {
			if strings.Contains(v, bad) {
				t.Errorf("第 %d 卦 %s 的卦辞白话含有不宜的措辞 %q", h.ID, h.Name, bad)
			}
		}
		if prev, dup := seen[v]; dup {
			t.Errorf("第 %d 卦与第 %d 卦的卦辞白话完全相同", h.ID, prev)
		}
		seen[v] = h.ID
	}
	if len(anchors) != 64 {
		t.Errorf("关键词表应覆盖 64 卦，实际 %d", len(anchors))
	}
}

func TestKanJudgementKeepsXiKan(t *testing.T) {
	// 源数据曾把「习坎」当作卦名而漏在卦辞之外；通行本卦辞是「习坎，有孚，维心亨，行有尚」
	h, _ := MustLoad().Get("010010")
	if !strings.HasPrefix(h.Judgement, "习坎，") {
		t.Errorf("坎卦卦辞应以「习坎，」开头，实为 %q", h.Judgement)
	}
}
