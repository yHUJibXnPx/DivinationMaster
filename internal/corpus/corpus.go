// Package corpus 加载内嵌的 iching.json（64 卦 / 384 爻 + 用九用六 / 象传 / 白话）并提供查询。
//
// iching.json 是唯一的语料来源（go:embed 编译进二进制）。来源与修订记录见其 meta 字段。
package corpus

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"divinationmaster/internal/hexagram"
)

//go:embed iching.json
var raw []byte

// Line 是一爻的经传文字。
type Line struct {
	Pos   int    `json:"pos"`
	Name  string `json:"name"`
	Text  string `json:"text"`
	Xiang string `json:"xiang"`
}

// Extra 是乾「用九」/ 坤「用六」。
type Extra struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Xiang string `json:"xiang"`
}

// Hexagram 是一卦的全部文字。
type Hexagram struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	FullName       string `json:"full_name"`
	Pinyin         string `json:"pinyin"`
	Symbol         string `json:"symbol"`
	Lower          string `json:"lower"`
	Upper          string `json:"upper"`
	Judgement      string `json:"judgement"`
	XiangImage     string `json:"xiang_image"`
	Interpretation string `json:"interpretation"`
	Lines          []Line `json:"lines"`
	Extra          *Extra `json:"extra,omitempty"`
}

// IChing 是加载后的语料。
type IChing struct {
	Hexagrams map[string]*Hexagram
	byID      map[int]string
	byName    map[string]string
}

// Load 解析内嵌语料并建立索引。
func Load() (*IChing, error) {
	var doc struct {
		Hexagrams map[string]*Hexagram `json:"hexagrams"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("解析内嵌语料失败: %w", err)
	}
	ic := &IChing{Hexagrams: doc.Hexagrams, byID: map[int]string{}, byName: map[string]string{}}
	for code, h := range ic.Hexagrams {
		if err := hexagram.CheckCode(code); err != nil {
			return nil, err
		}
		if len(h.Lines) != hexagram.NLines {
			return nil, fmt.Errorf("第 %d 卦爻数不是 6", h.ID)
		}
		ic.byID[h.ID] = code
		for _, key := range []string{h.Name, h.FullName, strings.TrimSuffix(h.FullName, "卦")} {
			ic.byName[key] = code
		}
	}
	if len(ic.Hexagrams) != 64 {
		return nil, fmt.Errorf("卦数不是 64: %d", len(ic.Hexagrams))
	}
	return ic, nil
}

// MustLoad 在语料损坏时 panic（语料是编译期内嵌的，损坏属于构建错误）。
func MustLoad() *IChing {
	ic, err := Load()
	if err != nil {
		panic(err)
	}
	return ic
}

// Get 按卦码取卦。
func (ic *IChing) Get(code string) (*Hexagram, error) {
	h, ok := ic.Hexagrams[code]
	if !ok {
		return nil, fmt.Errorf("未知卦码 %q", code)
	}
	return h, nil
}

// CodeOfID 按序号（1–64）取卦码。
func (ic *IChing) CodeOfID(id int) (string, bool) {
	c, ok := ic.byID[id]
	return c, ok
}

// Find 按 卦码 / 序号 / 卦名 / 全名 查找，返回卦码。
func (ic *IChing) Find(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if _, ok := ic.Hexagrams[key]; ok {
		return key, true
	}
	if n, err := strconv.Atoi(key); err == nil {
		if c, ok := ic.byID[n]; ok {
			return c, true
		}
	}
	c, ok := ic.byName[key]
	return c, ok
}

// Name 返回卦名（未知卦码返回空串）。
func (ic *IChing) Name(code string) string {
	if h, ok := ic.Hexagrams[code]; ok {
		return h.Name
	}
	return ""
}
