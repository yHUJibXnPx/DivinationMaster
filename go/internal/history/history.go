// Package history 是追加写入的 JSON Lines 历史记录（与 Python 版格式互通）。
package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"divinationmaster/internal/core"
)

// DefaultPath 是默认历史文件名。
const DefaultPath = "history.jsonl"

// ResolvePath：显式路径 > 环境变量 DIVINATION_HISTORY > 默认。
func ResolvePath(path string) string {
	if path != "" {
		return path
	}
	if env := os.Getenv("DIVINATION_HISTORY"); env != "" {
		return env
	}
	return DefaultPath
}

// Append 追加一条记录并 fsync（崩溃/断电最多丢最后一行）。
func Append(path string, rec *core.Result) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // 与 Python ensure_ascii=False 一致：不转义 < > &
	if err := enc.Encode(rec); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Load 读取全部记录；不是合法 JSON 的行会跳过并向 warn 输出警告。文件不存在返回空。
func Load(path string, warn io.Writer) ([]*core.Result, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []*core.Result
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for no := 1; sc.Scan(); no++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec core.Result
		if err := json.Unmarshal(line, &rec); err != nil {
			fmt.Fprintf(warn, "警告：%s 第 %d 行不是合法 JSON，已跳过\n", path, no)
			continue
		}
		out = append(out, &rec)
	}
	return out, sc.Err()
}
