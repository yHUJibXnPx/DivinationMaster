import unittest

from divination.hexagram import (
    DIM, PERM_CUO, PERM_ZONG, changed_code_of, changing_positions, code_of, code_to_index,
    cuo, hu, index_to_code, is_changing, line_bit, zong,
)


class HexagramTests(unittest.TestCase):
    def test_line_values(self):
        self.assertEqual([line_bit(v) for v in (6, 7, 8, 9)], [0, 1, 0, 1])
        self.assertEqual([is_changing(v) for v in (6, 7, 8, 9)], [True, False, False, True])
        with self.assertRaises(ValueError):
            line_bit(5)

    def test_codes(self):
        lines = [9, 8, 7, 6, 8, 7]
        self.assertEqual(code_of(lines), "101001")
        self.assertEqual(changed_code_of(lines), "001101")
        self.assertEqual(changing_positions(lines), [1, 4])
        self.assertEqual(changed_code_of([7, 8, 7, 8, 7, 8]), "101010")  # 无变爻 → 变卦=本卦

    def test_index_roundtrip(self):
        for i in range(DIM):
            self.assertEqual(code_to_index(index_to_code(i)), i)
        self.assertEqual(code_to_index("100000"), 1)  # 初爻 = 最低位

    def test_derived(self):
        self.assertEqual(cuo("100010"), "011101")   # 屯 → 鼎
        self.assertEqual(zong("100010"), "010001")  # 屯 → 蒙
        self.assertEqual(hu("100010"), "000001")    # 屯的互卦是剥
        self.assertEqual(hu("111111"), "111111")

    def test_involutions_and_permutations(self):
        for i in range(DIM):
            c = index_to_code(i)
            self.assertEqual(cuo(cuo(c)), c)
            self.assertEqual(zong(zong(c)), c)
            self.assertEqual(PERM_CUO[PERM_CUO[i]], i)
            self.assertEqual(PERM_ZONG[PERM_ZONG[i]], i)
        self.assertEqual(sorted(PERM_CUO), list(range(DIM)))   # 双射 → 可以是酉算子
        self.assertEqual(sorted(PERM_ZONG), list(range(DIM)))

    def test_hu_is_not_a_bijection(self):
        # 互卦 64 → 16，多对一，因此不能作为酉算子作用于态向量
        images = {hu(index_to_code(i)) for i in range(DIM)}
        self.assertEqual(len(images), 16)


if __name__ == "__main__":
    unittest.main()
