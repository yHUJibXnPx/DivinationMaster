"""终端文本渲染（只负责排版，不含逻辑）。"""
from __future__ import annotations

from .corpus import IChing
from .hexagram import N_LINES, render_lines

BAR = 28


def _hex_title(h: dict) -> str:
    return f"第{h['id']}卦 {h['symbol']} {h['name']}（{h['pinyin']}）· {h['full_name']}"


def render_hexagram(iching: IChing, code: str, changing=(), entanglement=None) -> str:
    h = iching.get(code)
    rows = render_lines(code, changing)
    if entanglement is not None:  # 自上而下与 rows 对齐
        rows = [f"{r}   纠缠 {entanglement[N_LINES - 1 - i]:.2f}" for i, r in enumerate(rows)]
    out = [_hex_title(h), *("  " + r for r in rows)]
    out.append(f"  卦辞：{h['judgement']}")
    out.append(f"  象曰：{h['xiang_image']}")
    out.append(f"  白话：{h['interpretation']}")
    return "\n".join(out)


def render_show(iching: IChing, code: str) -> str:
    h = iching.get(code)
    out = [render_hexagram(iching, code), ""]
    for ln in reversed(h["lines"]):
        out.append(f"  {ln['name']}：{ln['text']}")
        out.append(f"      象曰：{ln['xiang']}")
    if "extra" in h:
        e = h["extra"]
        out.append(f"  {e['name']}：{e['text']}")
        out.append(f"      象曰：{e['xiang']}")
    return "\n".join(out)


def render_divination(iching: IChing, r: dict) -> str:
    q = r["quantum"]
    out = []
    out.append(f"问题：{r['question'] or '（未填写）'}")
    out.append(f"种子：{r['seed']}（相同问题 + 相同种子 + 相同参数可完全复现）")
    out.append("")
    out.append("【本卦】")
    out.append(render_hexagram(iching, r["benguo"]["code"], r["changing"], q["line_entanglement"]))
    if r["biangua"]:
        out.append("")
        out.append("【变卦】")
        out.append(render_hexagram(iching, r["biangua"]["code"]))
    d = r["derived"]
    out.append("")
    out.append(f"【卦变】错卦：{d['cuo']['name']}  综卦：{d['zong']['name']}  互卦：{d['hu']['name']}")

    out.append("")
    out.append(f"【断卦】{r['reading']['rule']}")
    for p in r["reading"]["passages"]:
        star = "★" if p["primary"] else " "
        out.append(f" {star} {p['source']}·{p['label']}（{p['kind']}）：{p['text']}")
        out.append(f"       象曰：{p['xiang']}")

    out.append("")
    out.append("【量子视图】")
    out.append(f"  分布熵 {q['entropy_bits']:.2f} / 6 比特（0 = 完全确定，6 = 64 卦均等）")
    out.append("  测量前概率最大的卦：")
    for t in q["top"]:
        bar = "█" * max(1, round(t["p"] * BAR)) if t["p"] > 0 else ""
        out.append(f"    {t['name']:　<2}  {t['p'] * 100:5.1f}%  {bar}")
    m = q["measured"]
    out.append(f"  本次测量坍缩为：第{m['id']}卦 {m['name']} —— {q['relation']}")
    out.append("")
    out.append("  注：以上是对六比特系统的数学模拟，用于呈现可能性的结构，不构成预测。")
    return "\n".join(out)


def render_history_row(rec: dict) -> str:
    ben = rec["benguo"]["name"]
    bian = f" → {rec['biangua']['name']}" if rec.get("biangua") else ""
    meas = rec["quantum"]["measured"]["name"]
    return f"{rec['time']}  {ben}{bian}  （观测：{meas}）  {rec['question']}"
