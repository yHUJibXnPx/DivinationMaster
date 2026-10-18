package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

func TestAskTextNoSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.jsonl")
	code, out, _ := run(t, "我该接受这份工作吗？", "--seed", "7", "--no-save", "--history", path)
	if code != 0 || !strings.Contains(out, "【本卦】") || !strings.Contains(out, "【断卦】") || !strings.Contains(out, "【量子视图】") {
		t.Fatalf("code=%d out=%s", code, out)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("--no-save 不应创建历史文件")
	}
}

func TestAskSavesAndHistoryLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.jsonl")
	run(t, "ask", "问题一", "--seed", "1", "--history", path)
	run(t, "ask", "问题二", "--seed", "2", "--history", path)
	code, out, _ := run(t, "history", "--history", path, "-n", "5")
	if code != 0 || !strings.Contains(out, "问题一") || !strings.Contains(out, "问题二") {
		t.Fatalf("code=%d out=%s", code, out)
	}
	b, _ := os.ReadFile(path)
	if n := strings.Count(strings.TrimSpace(string(b)), "\n") + 1; n != 2 {
		t.Errorf("历史行数 %d，应为 2", n)
	}
}

func TestJSONOutput(t *testing.T) {
	code, out, _ := run(t, "ask", "q", "--seed", "3", "--json", "--no-save")
	var r struct{ Seed uint64 }
	if code != 0 || json.Unmarshal([]byte(out), &r) != nil || r.Seed != 3 {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestShow(t *testing.T) {
	if code, out, _ := run(t, "show", "屯"); code != 0 || !strings.Contains(out, "初九") || !strings.Contains(out, "上六") {
		t.Errorf("show 屯: %d %s", code, out)
	}
	if _, out, _ := run(t, "show", "乾"); !strings.Contains(out, "用九") {
		t.Error("show 乾 应含用九")
	}
	if code, _, _ := run(t, "show", "不存在的卦"); code != 1 {
		t.Errorf("找不到应返回 1，得 %d", code)
	}
}

func TestBadArguments(t *testing.T) {
	for _, args := range [][]string{
		{"ask", "q", "--fuzz", "5", "--no-save"},
		{"ask", "q", "--seed", "-1", "--no-save"},
		{"ask", "q", "--seed", "18446744073709551616", "--no-save"},
		{"ask", "q", "--seed", "abc", "--no-save"},
		{"ask", "两个", "问题", "--no-save"},
		{"ask", "q", "--nosuchflag"},
	} {
		if code, _, _ := run(t, args...); code != 2 {
			t.Errorf("%v 应返回 2，得 %d", args, code)
		}
	}
}

func TestOptionsBeforeQuestion(t *testing.T) {
	code, out, _ := run(t, "--seed", "9", "--no-save", "问题")
	if code != 0 || !strings.Contains(out, "种子：9") {
		t.Errorf("code=%d out=%s", code, out)
	}
}

func TestInteractiveQuestion(t *testing.T) {
	var out, errb bytes.Buffer
	code := Main([]string{"--seed", "1", "--no-save"}, strings.NewReader("  交互问题  \n"), &out, &errb)
	if code != 0 || !strings.Contains(out.String(), "问题：交互问题") {
		t.Errorf("code=%d out=%s", code, out.String())
	}
}

func TestCorruptHistoryLineSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.jsonl")
	run(t, "ask", "好的", "--seed", "1", "--history", path)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("{坏掉的行\n")
	f.Close()
	code, out, errs := run(t, "history", "--history", path)
	if code != 0 || !strings.Contains(out, "好的") || !strings.Contains(errs, "已跳过") {
		t.Errorf("code=%d out=%s err=%s", code, out, errs)
	}
}

func TestEntanglementLineOnlyWhenPresent(t *testing.T) {
	// seed 8：初爻与四爻同为变爻（应位）→ 显示纠缠；seed 25：静卦 → 明确写"无"，且不列 0% 的卦
	_, out, _ := run(t, "q", "--seed", "8", "--no-save")
	if !strings.Contains(out, "应位纠缠：初↔四") {
		t.Errorf("seed 8 应显示初↔四纠缠:\n%s", out)
	}
	_, out, _ = run(t, "q", "--seed", "25", "--no-save")
	if !strings.Contains(out, "应位纠缠：无") || strings.Contains(out, "  0.0%") {
		t.Errorf("seed 25 应写\"无\"且不列零概率卦:\n%s", out)
	}
}

func TestShowDisplaysVernacularOnlyWhereItExists(t *testing.T) {
	_, out, _ := run(t, "show", "乾")
	if !strings.Contains(out, "      白话：龙还潜伏在深渊里") || !strings.Contains(out, "用九：见群龙无首，吉。\n      白话：") {
		t.Errorf("乾应逐爻显示白话（含用九）:\n%s", out)
	}
	// 暂无白话的卦：只有整卦的「  白话：」，不应出现爻级的空白话行
	// 未济是传统顺序的最后一卦，最晚才会补白话；等它也补完时，这条断言就该删掉
	_, out, _ = run(t, "show", "未济")
	if strings.Contains(out, "      白话：") {
		t.Errorf("暂无白话的卦不应出现爻级白话行:\n%s", out)
	}
}
