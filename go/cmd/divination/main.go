// 命令 divination：周易占卜 · 六比特态向量引擎（Go 版，单个静态二进制，语料已内嵌）。
package main

import (
	"os"

	"divinationmaster/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
