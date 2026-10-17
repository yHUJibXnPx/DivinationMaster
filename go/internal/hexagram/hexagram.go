// Package hexagram 提供六爻、卦码与卦变算子（纯函数，无 I/O）。
//
// 约定（与 Python 版、旧版 hexagram.json 一致）：
//   - 卦码：6 位 '0'/'1' 字符串，自初爻到上爻（自下而上），1=阳、0=阴。
//   - 态向量下标：卦码反转后按二进制读出，即下标第 i 位（低位起）= 第 i+1 爻。
//   - 四态爻（三枚铜钱之和）：6 老阴、7 少阳、8 少阴、9 老阳；老阴/老阳是"变爻"。
package hexagram

import (
	"fmt"
	"strings"
)

const (
	OldYin    = 6
	YoungYang = 7
	YoungYin  = 8
	OldYang   = 9

	NLines = 6
	Dim    = 1 << NLines // 64
)

var positions = [NLines]string{"初", "二", "三", "四", "五", "上"}

// Lines 是自初爻起的六个四态爻。
type Lines [NLines]int

// ValidateLine 检查爻值是否为 6/7/8/9。
func ValidateLine(v int) error {
	if v < OldYin || v > OldYang {
		return fmt.Errorf("爻值必须是 6/7/8/9，收到 %d", v)
	}
	return nil
}

// Validate 检查六个爻值。
func (l Lines) Validate() error {
	for _, v := range l {
		if err := ValidateLine(v); err != nil {
			return err
		}
	}
	return nil
}

// LineBit 把四态爻转为本卦中的阴阳（1 阳 / 0 阴）。前置条件：v ∈ {6,7,8,9}。
func LineBit(v int) int {
	if v == YoungYang || v == OldYang {
		return 1
	}
	return 0
}

// IsChanging 判断是否变爻（老阴/老阳）。
func IsChanging(v int) bool { return v == OldYin || v == OldYang }

// CodeOf 返回本卦卦码。
func CodeOf(l Lines) string {
	var b strings.Builder
	for _, v := range l {
		b.WriteByte(byte('0' + LineBit(v)))
	}
	return b.String()
}

// ChangedCodeOf 返回变卦卦码：变爻阴阳互换，其余不变。
func ChangedCodeOf(l Lines) string {
	var b strings.Builder
	for _, v := range l {
		bit := LineBit(v)
		if IsChanging(v) {
			bit ^= 1
		}
		b.WriteByte(byte('0' + bit))
	}
	return b.String()
}

// ChangingPositions 返回变爻位置，1 起（1=初爻 … 6=上爻）。
func ChangingPositions(l Lines) []int {
	out := []int{}
	for i, v := range l {
		if IsChanging(v) {
			out = append(out, i+1)
		}
	}
	return out
}

// LineName 返回爻名，pos0 为 0 起的爻位；如 初九、六二、上六。
func LineName(pos0, bit int) string {
	n := "六"
	if bit == 1 {
		n = "九"
	}
	if pos0 == 0 || pos0 == NLines-1 {
		return positions[pos0] + n
	}
	return n + positions[pos0]
}

// CheckCode 校验卦码格式。
func CheckCode(code string) error {
	if len(code) != NLines {
		return fmt.Errorf("卦码必须是 6 位 0/1 字符串，收到 %q", code)
	}
	for i := 0; i < len(code); i++ {
		if code[i] != '0' && code[i] != '1' {
			return fmt.Errorf("卦码必须是 6 位 0/1 字符串，收到 %q", code)
		}
	}
	return nil
}

// CodeToIndex 卦码 → 态向量下标（初爻 = 最低位）。
func CodeToIndex(code string) (int, error) {
	if err := CheckCode(code); err != nil {
		return 0, err
	}
	idx := 0
	for i := 0; i < NLines; i++ {
		if code[i] == '1' {
			idx |= 1 << i
		}
	}
	return idx, nil
}

// IndexToCode 态向量下标 → 卦码。下标越界时 panic（属于程序错误）。
func IndexToCode(index int) string {
	if index < 0 || index >= Dim {
		panic(fmt.Sprintf("下标越界: %d", index))
	}
	b := make([]byte, NLines)
	for i := 0; i < NLines; i++ {
		b[i] = byte('0' + (index>>i)&1)
	}
	return string(b)
}

// mustIndex 用于已知合法的卦码。
func mustIndex(code string) int {
	i, err := CodeToIndex(code)
	if err != nil {
		panic(err)
	}
	return i
}

// Cuo 错卦：六爻阴阳全部互换（可逆，酉算子 X⊗6）。前置条件：code 合法。
func Cuo(code string) string {
	b := []byte(code)
	for i := range b {
		b[i] ^= 1 // '0'(0x30) <-> '1'(0x31)
	}
	return string(b)
}

// Zong 综卦：上下颠倒（可逆，比特序反转的置换算子）。前置条件：code 合法。
func Zong(code string) string {
	b := []byte(code)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// Hu 互卦：二三四爻为下卦、三四五爻为上卦。
// 这是多对一映射（64→16），不可逆，因此不是酉算子，只能作用于测量结果。
func Hu(code string) string { return code[1:4] + code[2:5] }

// PermCuo / PermZong 是作用在态向量下标上的置换（new[Perm[i]] = old[i]）。
var PermCuo, PermZong [Dim]int

func init() {
	for i := 0; i < Dim; i++ {
		c := IndexToCode(i)
		PermCuo[i] = mustIndex(Cuo(c))
		PermZong[i] = mustIndex(Zong(c))
	}
}

// RenderLines 返回文本卦象（自上而下）；changing 为 1 起的变爻位置，阳变 ○、阴变 ×。
func RenderLines(code string, changing []int) []string {
	isChg := map[int]bool{}
	for _, p := range changing {
		isChg[p] = true
	}
	rows := make([]string, 0, NLines)
	for pos0 := NLines - 1; pos0 >= 0; pos0-- {
		yang := code[pos0] == '1'
		body, bit := "━━━  ━━━", 0
		if yang {
			body, bit = "━━━━━━━━", 1
		}
		mark := ""
		if isChg[pos0+1] {
			if yang {
				mark = " ○"
			} else {
				mark = " ×"
			}
		}
		rows = append(rows, LineName(pos0, bit)+"  "+body+mark)
	}
	return rows
}
