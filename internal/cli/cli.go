// Package cli 是命令行入口：
//
//	divination "我该接受这份工作吗？"          # 等价于 ask
//	divination ask "问题" --seed 42 --json
//	divination show 屯                          # 按卦名/序号/卦码查看一整卦
//	divination history -n 10
package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"divinationmaster/internal/core"
	"divinationmaster/internal/corpus"
	"divinationmaster/internal/history"
	"divinationmaster/internal/quantum"
	"divinationmaster/internal/render"
)

const usage = `周易占卜 · 六比特态向量引擎

用法：
  divination [ask] [问题] [选项]   起卦并解读（默认命令；省略问题则交互输入）
  divination show <卦名|序号|卦码> 查看一整卦（卦辞、六爻爻辞、象传）
  divination history [-n N]        查看历史记录

ask 选项：--seed N  --fuzz X  --entangle X  --interfere X  --top N  --history PATH  --no-save  --json
`

// Main 运行命令行，返回退出码。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "ask" && args[0] != "show" && args[0] != "history" && args[0] != "-h" && args[0] != "--help") {
		args = append([]string{"ask"}, args...) // 第一个参数不是子命令 → 当作 ask 的参数
	}
	switch args[0] {
	case "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "ask":
		return runAsk(args[1:], stdin, stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	default:
		return runHistory(args[1:], stdout, stderr)
	}
}

// parseInterspersed 允许选项与位置参数交错（flag 包默认遇到第一个位置参数就停止解析）。
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func runAsk(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	def := quantum.DefaultConfig()
	fs := newFlagSet("ask", stderr)
	seedStr := fs.String("seed", "", "随机种子 0..2^64-1；省略则取系统熵（结果会打印出来，可复现）")
	fuzz := fs.Float64("fuzz", def.Fuzz, "稳定爻的涨落角，0..π/4")
	entangle := fs.Float64("entangle", def.Entangle, "应位纠缠角，0=不纠缠")
	interfere := fs.Float64("interfere", def.Interfere, "干涉角，0=不干涉")
	top := fs.Int("top", 5, "显示概率最大的前 N 个卦")
	histPath := fs.String("history", "", "历史文件路径（默认 ./history.jsonl 或环境变量 DIVINATION_HISTORY）")
	noSave := fs.Bool("no-save", false, "不写入历史")
	asJSON := fs.Bool("json", false, "输出 JSON 而不是文本")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(pos) > 1 {
		fmt.Fprintf(stderr, "多余的参数：%s（问题含空格时请加引号）\n", strings.Join(pos[1:], " "))
		return 2
	}

	var seed uint64
	if *seedStr == "" {
		if seed, err = core.NewSeed(); err != nil {
			fmt.Fprintf(stderr, "无法获取系统熵：%v\n", err)
			return 1
		}
	} else if seed, err = strconv.ParseUint(*seedStr, 10, 64); err != nil {
		fmt.Fprintf(stderr, "种子必须是 0 .. 2^64-1 之间的整数: %q\n", *seedStr)
		return 2
	}
	cfg := quantum.Config{Fuzz: *fuzz, Entangle: *entangle, Interfere: *interfere}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "参数错误：%v\n", err)
		return 2
	}

	var question string
	if len(pos) == 1 {
		question = pos[0]
	} else {
		fmt.Fprint(stdout, "请输入你的问题：")
		line, _ := bufio.NewReader(stdin).ReadString('\n') // EOF → 空问题
		question = strings.TrimSpace(line)
	}

	ic, err := corpus.Load()
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	topK := *top
	if topK < 1 {
		topK = 1
	}
	res, err := core.Divine(question, seed, cfg, ic, topK)
	if err != nil {
		fmt.Fprintf(stderr, "占卜失败：%v\n", err)
		return 1
	}
	if *asJSON {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		stdout.Write(buf.Bytes())
	} else {
		fmt.Fprintln(stdout, render.Divination(ic, res))
	}
	if !*noSave {
		if err := history.Append(history.ResolvePath(*histPath), res); err != nil {
			fmt.Fprintf(stderr, "写入历史失败：%v\n", err)
			return 1
		}
	}
	return 0
}

func runShow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("show", stderr)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, "用法：divination show <卦名|序号|卦码>")
		return 2
	}
	ic, err := corpus.Load()
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	code, ok := ic.Find(pos[0])
	if !ok {
		fmt.Fprintf(stderr, "找不到：%q（可用卦名、1–64 的序号或 6 位卦码）\n", pos[0])
		return 1
	}
	fmt.Fprintln(stdout, render.Show(ic, code))
	return 0
}

func runHistory(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("history", stderr)
	histPath := fs.String("history", "", "历史文件路径")
	n := fs.Int("n", 10, "最近 N 条")
	if _, err := parseInterspersed(fs, args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	recs, err := history.Load(history.ResolvePath(*histPath), stderr)
	if err != nil {
		fmt.Fprintf(stderr, "读取历史失败：%v\n", err)
		return 1
	}
	if len(recs) == 0 {
		fmt.Fprintln(stdout, "（还没有历史记录）")
		return 0
	}
	if *n < 1 {
		*n = 1
	}
	if len(recs) > *n {
		recs = recs[len(recs)-*n:]
	}
	for _, r := range recs {
		fmt.Fprintln(stdout, render.HistoryRow(r))
	}
	return 0
}
