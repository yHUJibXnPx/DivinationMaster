import math
import unittest

from divination.hexagram import code_of, code_to_index, index_to_code
from divination.rng import Rng
from divination.quantum import EngineConfig, State, YING_PAIRS, prepare, run_engine

CFG0 = EngineConfig(fuzz=0.0, entangle=math.pi / 2, interfere=0.0)


class QuantumTests(unittest.TestCase):
    def test_normalised_everywhere(self):
        for lines in ([7, 8, 7, 8, 7, 8], [9, 6, 9, 6, 9, 6], [9, 7, 7, 6, 8, 7]):
            st = run_engine(lines, "q", 1, EngineConfig())
            self.assertAlmostEqual(st.norm2(), 1.0, places=12)
            self.assertAlmostEqual(sum(st.probabilities()), 1.0, places=12)

    def test_no_change_no_fuzz_is_deterministic(self):
        lines = [7, 8, 7, 8, 7, 8]
        st = run_engine(lines, "q", 1, CFG0)
        self.assertEqual(st.support(), [code_to_index(code_of(lines))])
        self.assertAlmostEqual(st.entropy(), 0.0, places=12)

    def test_support_is_subcube_of_changing_lines(self):
        # 无涨落/无干涉：可能的卦恰是"变爻任意子集翻转"的 2^k 个，且等概率
        lines = [9, 7, 6, 8, 9, 7]
        st = run_engine(lines, "q", 5, CFG0)
        ben = code_of(lines)
        changing = [i for i, v in enumerate(lines) if v in (6, 9)]
        expected = set()
        for mask in range(1 << len(changing)):
            c = list(ben)
            for j, pos in enumerate(changing):
                if (mask >> j) & 1:
                    c[pos] = "1" if c[pos] == "0" else "0"
            expected.add(code_to_index("".join(c)))
        self.assertEqual(set(st.support()), expected)
        for i in expected:
            self.assertAlmostEqual(st.probabilities()[i], 1 / len(expected), places=12)
        self.assertAlmostEqual(st.entropy(), len(changing), places=9)

    def test_entanglement_zero_without_cphase(self):
        st = run_engine([9, 6, 9, 6, 9, 6], "q", 3, EngineConfig(fuzz=0.2, entangle=0.0, interfere=0.35))
        for q in range(6):
            self.assertAlmostEqual(st.line_entanglement(q), 0.0, places=9)

    def test_entanglement_between_ying_pairs(self):
        # 初爻(老阳)与四爻(老阴)互为应位，且都处于叠加态 → 应纠缠；其余爻稳定 → 不纠缠
        lines = [9, 7, 7, 6, 7, 7]
        st = run_engine(lines, "q", 3, EngineConfig(fuzz=0.0, entangle=math.pi / 2, interfere=0.0))
        self.assertGreater(st.line_entanglement(0), 0.5)
        self.assertGreater(st.line_entanglement(3), 0.5)
        for q in (1, 2, 4, 5):
            self.assertAlmostEqual(st.line_entanglement(q), 0.0, places=9)
        self.assertIn((0, 3), YING_PAIRS)

    def test_entanglement_invariant_under_local_interference(self):
        # 干涉层是逐比特的局域门，不应改变纠缠熵
        lines = [9, 7, 7, 6, 7, 7]
        a = run_engine(lines, "q", 3, EngineConfig(fuzz=0.0, interfere=0.0))
        b = run_engine(lines, "q", 3, EngineConfig(fuzz=0.0, interfere=0.7))
        for q in range(6):
            self.assertAlmostEqual(a.line_entanglement(q), b.line_entanglement(q), places=9)

    def test_question_changes_distribution_via_interference(self):
        lines = [9, 7, 7, 6, 7, 7]
        cfg = EngineConfig()
        p1 = run_engine(lines, "问题甲", 11, cfg).probabilities()
        p2 = run_engine(lines, "问题乙", 11, cfg).probabilities()
        self.assertGreater(max(abs(a - b) for a, b in zip(p1, p2)), 1e-4)
        # 无干涉时问题的相位不改变测量概率
        q1 = run_engine(lines, "问题甲", 11, CFG0).probabilities()
        q2 = run_engine(lines, "问题乙", 11, CFG0).probabilities()
        self.assertLess(max(abs(a - b) for a, b in zip(q1, q2)), 1e-12)

    def test_reproducible(self):
        a = run_engine([9, 7, 8, 6, 7, 8], "q", 99).probabilities()
        b = run_engine([9, 7, 8, 6, 7, 8], "q", 99).probabilities()
        self.assertEqual(a, b)

    def test_cuo_zong_are_unitary_permutations(self):
        st = run_engine([9, 7, 8, 6, 7, 8], "q", 4)
        for op in (st.cuo, st.zong):
            out = op()
            self.assertAlmostEqual(out.norm2(), 1.0, places=12)
            self.assertEqual(sorted(out.probabilities()), sorted(st.probabilities()))
        self.assertEqual(st.cuo().cuo().amps, st.amps)
        self.assertEqual(st.zong().zong().amps, st.amps)
        self.assertEqual(State.basis("100010").zong().support(), [code_to_index("010001")])

    def test_measure_follows_born_rule(self):
        st = prepare([9, 7, 7, 7, 7, 7], "q", 1, EngineConfig(fuzz=0.0))  # 约 50/50
        rng = Rng(123)
        n = 4000
        hits = sum(st.measure(rng) == code_to_index("111111") for _ in range(n))
        self.assertAlmostEqual(hits / n, 0.5, delta=0.04)

    def test_config_validation(self):
        with self.assertRaises(ValueError):
            EngineConfig(fuzz=-0.1)
        with self.assertRaises(ValueError):
            EngineConfig(fuzz=1.0)
        with self.assertRaises(ValueError):
            EngineConfig(entangle=float("nan"))


if __name__ == "__main__":
    unittest.main()
