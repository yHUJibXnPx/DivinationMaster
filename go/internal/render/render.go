// Package render 负责终端文本排版（不含逻辑）。输出与 Python 版 render.py 逐字一致。
package render

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"divinationmaster/internal/core"
	"divinationmaster/internal/corpus"
	"divinationmaster/internal/hexagram"
)

const bar = 28

func hexTitle(h *corpus.Hexagram) string {
	return fmt.Sprintf("第%d卦 %s %s（%s）· %s", h.ID, h.Symbol, h.Name, h.Pinyin, h.FullName)
}

// padRunes 用 fill 把 s 按字符数右补齐到 width（对应 Python 的 f"{s:　<2}"）。
func padRunes(s string, width int, fill string) string {
	if n := utf8.RuneCountInString(s); n < width {
		return s + strings.Repeat(fill, width-n)
	}
	return s
}

// Hexagram 排版一卦；entanglement 非 nil 时在每爻后附纠缠熵。
func Hexagram(ic *corpus.IChing, code string, changing []int, entanglement []float64) string {
	h, _ := ic.Get(code)
	rows := hexagram.RenderLines(code, changing)
	if entanglement != nil {
		for i := range rows {
			rows[i] = fmt.Sprintf("%s   纠缠 %.2f", rows[i], entanglement[hexagram.NLines-1-i])
		}
	}
	out := []string{hexTitle(h)}
	for _, r := range rows {
		out = append(out, "  "+r)
	}
	out = append(out,
		"  卦辞："+h.Judgement,
		"  象曰："+h.XiangImage,
		"  白话："+h.Interpretation)
	return strings.Join(out, "\n")
}

// Show 排版一整卦：卦辞、六爻爻辞、象传。
func Show(ic *corpus.IChing, code string) string {
	h, _ := ic.Get(code)
	out := []string{Hexagram(ic, code, nil, nil), ""}
	for i := len(h.Lines) - 1; i >= 0; i-- {
		ln := h.Lines[i]
		out = append(out, fmt.Sprintf("  %s：%s", ln.Name, ln.Text), "      象曰："+ln.Xiang)
	}
	if h.Extra != nil {
		e := h.Extra
		out = append(out, fmt.Sprintf("  %s：%s", e.Name, e.Text), "      象曰："+e.Xiang)
	}
	return strings.Join(out, "\n")
}

// Divination 排版一次占卜。
func Divination(ic *corpus.IChing, r *core.Result) string {
	q := r.Quantum
	question := r.Question
	if question == "" {
		question = "（未填写）"
	}
	out := []string{
		"问题：" + question,
		fmt.Sprintf("种子：%d（相同问题 + 相同种子 + 相同参数可完全复现）", r.Seed),
		"",
		"【本卦】",
		Hexagram(ic, r.Benguo.Code, r.Changing, q.LineEntanglement),
	}
	if r.Biangua != nil {
		out = append(out, "", "【变卦】", Hexagram(ic, r.Biangua.Code, nil, nil))
	}
	d := r.Derived
	out = append(out, "",
		fmt.Sprintf("【卦变】错卦：%s  综卦：%s  互卦：%s", d.Cuo.Name, d.Zong.Name, d.Hu.Name),
		"",
		"【断卦】"+r.Reading.Rule)
	for _, p := range r.Reading.Passages {
		star := " "
		if p.Primary {
			star = "★"
		}
		out = append(out,
			fmt.Sprintf(" %s %s·%s（%s）：%s", star, p.Source, p.Label, p.Kind, p.Text),
			"       象曰："+p.Xiang)
	}
	out = append(out, "", "【量子视图】",
		fmt.Sprintf("  分布熵 %.2f / 6 比特（0 = 完全确定，6 = 64 卦均等）", q.EntropyBits),
		"  测量前概率最大的卦：")
	for _, t := range q.Top {
		b := ""
		if t.P > 0 {
			n := int(math.RoundToEven(t.P * bar))
			if n < 1 {
				n = 1
			}
			b = strings.Repeat("█", n)
		}
		out = append(out, fmt.Sprintf("    %s  %5.1f%%  %s", padRunes(t.Name, 2, "　"), t.P*100, b))
	}
	m := q.Measured
	out = append(out,
		fmt.Sprintf("  本次测量坍缩为：第%d卦 %s —— %s", m.ID, m.Name, q.Relation),
		"",
		"  注：以上是对六比特系统的数学模拟，用于呈现可能性的结构，不构成预测。")
	return strings.Join(out, "\n")
}

// HistoryRow 排版一条历史记录。
func HistoryRow(r *core.Result) string {
	bian := ""
	if r.Biangua != nil {
		bian = " → " + r.Biangua.Name
	}
	return fmt.Sprintf("%s  %s%s  （观测：%s）  %s", r.Time, r.Benguo.Name, bian, r.Quantum.Measured.Name, r.Question)
}
