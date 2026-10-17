import unittest
from collections import Counter

from divination.corpus import load
from divination.rng import Rng
from divination.reading import build_reading, cast_hexagram, cast_line, observation_relation


class CastTests(unittest.TestCase):
    def test_line_distribution_is_1_3_3_1(self):
        rng = Rng(2024)
        n = 16000
        c = Counter(cast_line(rng) for _ in range(n))
        self.assertEqual(set(c), {6, 7, 8, 9})
        for value, expected in ((6, 1 / 8), (7, 3 / 8), (8, 3 / 8), (9, 1 / 8)):
            self.assertAlmostEqual(c[value] / n, expected, delta=0.015)

    def test_cast_is_reproducible(self):
        self.assertEqual(cast_hexagram(Rng(5)), cast_hexagram(Rng(5)))
        self.assertEqual(len(cast_hexagram(Rng(5))), 6)


class ReadingRuleTests(unittest.TestCase):
    def setUp(self):
        self.ic = load()

    def summary(self, lines):
        r = build_reading(lines, self.ic)
        return r["n_changing"], [(p["source"], p["kind"], p["label"], p["primary"]) for p in r["passages"]]

    def test_zero_changing(self):
        n, ps = self.summary([7, 8, 7, 8, 7, 8])
        self.assertEqual((n, ps), (0, [("本卦", "卦辞", "既济", True)]))  # 101010 = 既济（离下坎上）

    def test_one_changing(self):
        n, ps = self.summary([9, 7, 7, 7, 7, 7])
        self.assertEqual((n, ps), (1, [("本卦", "爻辞", "初九", True)]))

    def test_two_changing_upper_primary(self):
        n, ps = self.summary([9, 7, 6, 7, 7, 7])
        self.assertEqual(n, 2)
        self.assertEqual(ps, [("本卦", "爻辞", "初九", False), ("本卦", "爻辞", "六三", True)])

    def test_three_changing(self):
        n, ps = self.summary([9, 9, 9, 7, 7, 7])
        self.assertEqual(n, 3)
        self.assertEqual([(p[0], p[1], p[3]) for p in ps], [("本卦", "卦辞", True), ("变卦", "卦辞", False)])

    def test_four_changing_reads_unchanged_lines_of_bian(self):
        n, ps = self.summary([9, 9, 9, 9, 7, 7])   # 本卦乾，变卦 001111 遁；不变爻 五、上
        self.assertEqual(n, 4)
        self.assertEqual(ps, [("变卦", "爻辞", "九五", True), ("变卦", "爻辞", "上九", False)])

    def test_five_changing(self):
        n, ps = self.summary([9, 9, 9, 9, 9, 7])
        self.assertEqual((n, ps), (5, [("变卦", "爻辞", "上九", True)]))

    def test_six_changing_qian_kun_use_extra(self):
        self.assertEqual(self.summary([9] * 6)[1], [("本卦", "爻辞", "用九", True)])
        self.assertEqual(self.summary([6] * 6)[1], [("本卦", "爻辞", "用六", True)])

    def test_six_changing_other_reads_bian_judgement(self):
        n, ps = self.summary([9, 6, 6, 6, 9, 6])   # 屯 → 鼎
        self.assertEqual(n, 6)
        self.assertEqual(ps, [("变卦", "卦辞", "鼎", True)])

    def test_every_passage_has_text(self):
        rng = Rng(1)
        for _ in range(300):
            for p in build_reading(cast_hexagram(rng), self.ic)["passages"]:
                self.assertTrue(p["text"].strip() and p["xiang"].strip())


class RelationTests(unittest.TestCase):
    def test_relations(self):
        lines = [9, 7, 6, 8, 7, 7]  # 本卦 111011? 位 1,3 变
        ben = "".join("1" if v in (7, 9) else "0" for v in lines)
        flip = lambda c, ps: "".join(("1" if ch == "0" else "0") if i + 1 in ps else ch for i, ch in enumerate(c))
        self.assertEqual(observation_relation(ben, lines), "与本卦一致")
        self.assertEqual(observation_relation(flip(ben, {1, 3}), lines), "与变卦一致")
        self.assertIn("中间态", observation_relation(flip(ben, {1}), lines))
        self.assertIn("涨落态", observation_relation(flip(ben, {2}), lines))


if __name__ == "__main__":
    unittest.main()
