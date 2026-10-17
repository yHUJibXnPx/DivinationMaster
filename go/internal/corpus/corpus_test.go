package corpus

import (
	"regexp"
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
