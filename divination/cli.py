"""命令行入口：
    python -m divination "我该接受这份工作吗？"      # 等价于 ask
    python -m divination ask "问题" --seed 42 --json
    python -m divination show 屯                      # 按卦名/序号/卦码查看一整卦
    python -m divination history -n 10
"""
from __future__ import annotations

import argparse
import json
import sys

from . import history
from .corpus import load
from .core import divine, new_seed
from .quantum import EngineConfig
from .rng import SEED_MAX
from .render import render_divination, render_history_row, render_show

COMMANDS = {"ask", "show", "history"}


def _seed(text: str) -> int:
    try:
        v = int(text)
    except ValueError:
        raise argparse.ArgumentTypeError(f"种子必须是整数: {text!r}")
    if not 0 <= v <= SEED_MAX:
        raise argparse.ArgumentTypeError("种子必须在 0 .. 2^64-1 之间（Python 版与 Go 版共用同一范围）")
    return v


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="divination", description="周易占卜 · 六比特态向量引擎（v2）")
    sub = p.add_subparsers(dest="cmd")

    a = sub.add_parser("ask", help="起卦并解读（默认命令）")
    a.add_argument("question", nargs="?", help="问题；省略则交互输入")
    a.add_argument("--seed", type=_seed, help="随机种子；省略则取系统熵（结果会打印出来，可复现）")
    a.add_argument("--fuzz", type=float, default=EngineConfig.fuzz, help="稳定爻的涨落角，0..π/4")
    a.add_argument("--entangle", type=float, default=EngineConfig.entangle, help="应位纠缠角，0=不纠缠")
    a.add_argument("--interfere", type=float, default=EngineConfig.interfere, help="干涉角，0=不干涉")
    a.add_argument("--top", type=int, default=5, help="显示概率最大的前 N 个卦")
    a.add_argument("--history", help="历史文件路径（默认 ./history.jsonl 或环境变量 DIVINATION_HISTORY）")
    a.add_argument("--no-save", action="store_true", help="不写入历史")
    a.add_argument("--json", action="store_true", help="输出 JSON 而不是文本")

    s = sub.add_parser("show", help="查看一整卦（卦辞、六爻爻辞、象传）")
    s.add_argument("key", help="卦名、序号或 6 位卦码（自下而上，1=阳）")

    h = sub.add_parser("history", help="查看历史记录")
    h.add_argument("--history", help="历史文件路径")
    h.add_argument("-n", type=int, default=10, help="最近 N 条")
    return p


def _ask(args: argparse.Namespace) -> int:
    question = args.question
    if question is None:
        try:
            question = input("请输入你的问题：").strip()
        except EOFError:
            question = ""
    try:
        cfg = EngineConfig(fuzz=args.fuzz, entangle=args.entangle, interfere=args.interfere)
    except ValueError as e:
        print(f"参数错误：{e}", file=sys.stderr)
        return 2
    seed = new_seed() if args.seed is None else args.seed
    result = divine(question, seed=seed, cfg=cfg, top_k=max(1, args.top))

    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(render_divination(load(), result))
    if not args.no_save:
        history.append(history.resolve_path(args.history), result)
    return 0


def _show(args: argparse.Namespace) -> int:
    iching = load()
    code = iching.find(args.key)
    if code is None:
        print(f"找不到：{args.key!r}（可用卦名、1–64 的序号或 6 位卦码）", file=sys.stderr)
        return 1
    print(render_show(iching, code))
    return 0


def _history(args: argparse.Namespace) -> int:
    records = history.load(history.resolve_path(args.history))
    if not records:
        print("（还没有历史记录）")
        return 0
    for rec in records[-max(1, args.n):]:
        print(render_history_row(rec))
    return 0


def main(argv: list[str] | None = None) -> int:
    argv = list(sys.argv[1:] if argv is None else argv)
    if not argv or (argv[0] not in COMMANDS and argv[0] not in ("-h", "--help")):
        argv.insert(0, "ask")  # 第一个参数不是子命令 → 当作 ask 的参数（问题或选项）
    args = build_parser().parse_args(argv)
    return {"ask": _ask, "show": _show, "history": _history}[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
