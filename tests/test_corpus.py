import re
import unittest

from divination.corpus import IChing, load
from divination.hexagram import cuo, line_name, zong

TRIGRAM = {"111": "乾", "000": "坤", "100": "震", "011": "巽",
           "010": "坎", "101": "离", "001": "艮", "110": "兑"}


class CorpusTests(unittest.TestCase):
    def setUp(self):
        self.ic: IChing = load()

    def test_counts(self):
        self.assertEqual(len(self.ic.hexagrams), 64)
        self.assertEqual(sorted(h["id"] for h in self.ic.hexagrams.values()), list(range(1, 65)))
        n = sum(len(h["lines"]) + ("extra" in h) for h in self.ic.hexagrams.values())
        self.assertEqual(n, 386)  # 384 爻 + 用九 + 用六

    def test_only_qian_kun_have_extra(self):
        extras = {h["name"]: h["extra"]["name"] for h in self.ic.hexagrams.values() if "extra" in h}
        self.assertEqual(extras, {"乾": "用九", "坤": "用六"})

    def test_lines_consistent_with_code(self):
        for code, h in self.ic.hexagrams.items():
            self.assertEqual(len(h["lines"]), 6)
            for i, ln in enumerate(h["lines"]):
                self.assertEqual(ln["pos"], i + 1)
                self.assertEqual(ln["name"], line_name(i, int(code[i])), (code, ln))
                for k in ("text", "xiang"):
                    self.assertTrue(ln[k].strip(), (code, i, k))

    def test_trigrams_match_code(self):
        # 独立校验：由卦码推出的上下卦必须与语料一致（同时验证"自下而上"的约定）
        for code, h in self.ic.hexagrams.items():
            self.assertEqual((h["lower"], h["upper"]), (TRIGRAM[code[:3]], TRIGRAM[code[3:]]), code)

    def test_every_hexagram_has_text(self):
        for h in self.ic.hexagrams.values():
            for k in ("name", "full_name", "pinyin", "symbol", "judgement", "xiang_image", "interpretation"):
                self.assertTrue(h[k].strip(), (h["id"], k))

    def test_legacy_guan_bo_bug_fixed(self):
        self.assertEqual(self.ic.name("000011"), "观")
        self.assertEqual(self.ic.name("000001"), "剥")

    def test_xu_gua_pairs(self):
        # 序卦"非覆即变"：奇数序号的卦与下一卦互为综卦；综卦等于自身时互为错卦
        for odd in range(1, 64, 2):
            a, b = self.ic.code_of_id(odd), self.ic.code_of_id(odd + 1)
            partner = zong(a) if zong(a) != a else cuo(a)
            self.assertEqual(partner, b, (odd, self.ic.name(a), self.ic.name(b)))

    def test_interpretation_has_no_known_seams(self):
        # 旧白话在「摘要｜详解」接缝处漏标句号，如「得此卦者不可独立创业坤象征大地」
        for h in self.ic.hexagrams.values():
            m = re.search(r"得此卦者[^，。；：！？\n]{1,24}" + re.escape(h["name"]) +
                          r"(?:象征|是|的本意|的意思|代表|卦)", h["interpretation"])
            self.assertIsNone(m, (h["id"], h["name"], m and m.group(0)))

    def test_find(self):
        self.assertEqual(self.ic.find("屯"), "100010")
        self.assertEqual(self.ic.find("3"), "100010")
        self.assertEqual(self.ic.find("100010"), "100010")
        self.assertEqual(self.ic.find("水雷屯"), "100010")
        self.assertIsNone(self.ic.find("不存在"))


if __name__ == "__main__":
    unittest.main()
