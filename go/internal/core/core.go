// Package core 编排一次完整占卜：起卦 → 量子态演化 → 测量 → 断卦。
//
// 同一 (问题, 种子, 引擎参数) 的结果完全可复现。Result 的 JSON 结构与 Python 版一致，
// 因此两个版本写出的 history.jsonl 可以互相读取。
package core

import (
	crand "crypto/rand"
	"encoding/binary"
	"time"

	"divinationmaster/internal/corpus"
	"divinationmaster/internal/hexagram"
	"divinationmaster/internal/quantum"
	"divinationmaster/internal/reading"
	"divinationmaster/internal/rng"
)

// Ref 指向一卦。
type Ref struct {
	Code string `json:"code"`
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TopEntry 是测量前概率最大的卦之一。
type TopEntry struct {
	Code string  `json:"code"`
	ID   int     `json:"id"`
	Name string  `json:"name"`
	P    float64 `json:"p"`
}

// Derived 是本卦的传统卦变（错/综为酉变换，互为经典映射）。
type Derived struct {
	Cuo  Ref `json:"cuo"`
	Zong Ref `json:"zong"`
	Hu   Ref `json:"hu"`
}

// ConfigView 是写入记录的引擎参数。
type ConfigView struct {
	Fuzz      float64 `json:"fuzz"`
	Entangle  float64 `json:"entangle"`
	Interfere float64 `json:"interfere"`
}

// QuantumView 是量子视图。
type QuantumView struct {
	EntropyBits      float64    `json:"entropy_bits"`
	LineEntanglement []float64  `json:"line_entanglement"`
	Top              []TopEntry `json:"top"`
	Measured         Ref        `json:"measured"`
	Relation         string     `json:"relation"`
}

// Result 是一次占卜的完整结果（可直接写入 JSON）。
type Result struct {
	Time     string          `json:"time"`
	Question string          `json:"question"`
	Seed     uint64          `json:"seed"`
	Config   ConfigView      `json:"config"`
	Lines    []int           `json:"lines"`
	Changing []int           `json:"changing"`
	Benguo   Ref             `json:"benguo"`
	Biangua  *Ref            `json:"biangua"`
	Derived  Derived         `json:"derived"`
	Reading  reading.Reading `json:"reading"`
	Quantum  QuantumView     `json:"quantum"`
}

// NewSeed 从操作系统熵取一个 64 位种子。
func NewSeed() (uint64, error) {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

// Divine 做一次完整占卜。topK 为量子视图中保留的最大概率卦数。
func Divine(question string, seed uint64, cfg quantum.Config, ic *corpus.IChing, topK int) (*Result, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	r := rng.New(seed)
	lines := reading.CastHexagram(r)
	ben := hexagram.CodeOf(lines)
	changing := hexagram.ChangingPositions(lines)

	ref := func(code string) (Ref, error) {
		h, err := ic.Get(code)
		if err != nil {
			return Ref{}, err
		}
		return Ref{code, h.ID, h.Name}, nil
	}
	res := &Result{
		Time:     time.Now().Format("2006-01-02T15:04:05"),
		Question: question,
		Seed:     seed,
		Config:   ConfigView{cfg.Fuzz, cfg.Entangle, cfg.Interfere},
		Lines:    lines[:],
		Changing: changing,
	}
	var err error
	if res.Benguo, err = ref(ben); err != nil {
		return nil, err
	}
	if len(changing) > 0 {
		b, err := ref(hexagram.ChangedCodeOf(lines))
		if err != nil {
			return nil, err
		}
		res.Biangua = &b
	}
	if res.Derived.Cuo, err = ref(hexagram.Cuo(ben)); err != nil {
		return nil, err
	}
	if res.Derived.Zong, err = ref(hexagram.Zong(ben)); err != nil {
		return nil, err
	}
	if res.Derived.Hu, err = ref(hexagram.Hu(ben)); err != nil {
		return nil, err
	}
	if res.Reading, err = reading.Build(lines, ic); err != nil {
		return nil, err
	}

	state := quantum.Run(lines, question, seed, cfg)
	measured := hexagram.IndexToCode(state.Measure(r))
	qv := QuantumView{EntropyBits: state.Entropy(), LineEntanglement: make([]float64, hexagram.NLines)}
	for q := 0; q < hexagram.NLines; q++ {
		qv.LineEntanglement[q] = state.LineEntanglement(q)
	}
	for _, t := range state.Top(topK) {
		code := hexagram.IndexToCode(t.Index)
		h, err := ic.Get(code)
		if err != nil {
			return nil, err
		}
		qv.Top = append(qv.Top, TopEntry{code, h.ID, h.Name, t.P})
	}
	if qv.Measured, err = ref(measured); err != nil {
		return nil, err
	}
	qv.Relation = reading.ObservationRelation(measured, lines)
	res.Quantum = qv
	return res, nil
}
