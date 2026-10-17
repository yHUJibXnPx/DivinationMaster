import contextlib
import io
import json
import os
import tempfile
import unittest

from divination.cli import main
from divination.core import divine


def run(argv):
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        code = main(argv)
    return code, buf.getvalue()


class DivineTests(unittest.TestCase):
    def test_same_seed_same_result(self):
        a = divine("同一个问题", seed=42)
        b = divine("同一个问题", seed=42)
        for r in (a, b):
            r.pop("time")
        self.assertEqual(a, b)

    def test_different_seed_usually_differs(self):
        results = {tuple(divine("q", seed=s)["lines"]) for s in range(20)}
        self.assertGreater(len(results), 10)

    def test_result_is_json_serialisable_and_complete(self):
        r = json.loads(json.dumps(divine("q", seed=1), ensure_ascii=False))
        for k in ("lines", "changing", "benguo", "reading", "quantum", "derived", "config", "seed"):
            self.assertIn(k, r)
        self.assertEqual(len(r["quantum"]["line_entanglement"]), 6)
        self.assertAlmostEqual(sum(t["p"] for t in divine("q", seed=1, top_k=64)["quantum"]["top"]), 1.0, places=9)


class CliTests(unittest.TestCase):
    def test_ask_text_and_no_save(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "h.jsonl")
            code, out = run(["我该接受这份工作吗？", "--seed", "7", "--no-save", "--history", path])
            self.assertEqual(code, 0)
            self.assertIn("【本卦】", out)
            self.assertIn("【断卦】", out)
            self.assertIn("【量子视图】", out)
            self.assertFalse(os.path.exists(path))

    def test_ask_saves_and_history_lists(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "h.jsonl")
            run(["ask", "问题一", "--seed", "1", "--history", path])
            run(["ask", "问题二", "--seed", "2", "--history", path])
            code, out = run(["history", "--history", path, "-n", "5"])
            self.assertEqual(code, 0)
            self.assertIn("问题一", out)
            self.assertIn("问题二", out)
            with open(path, encoding="utf-8") as f:
                self.assertEqual(len([ln for ln in f if ln.strip()]), 2)

    def test_json_output(self):
        code, out = run(["ask", "q", "--seed", "3", "--json", "--no-save"])
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(out)["seed"], 3)

    def test_show(self):
        code, out = run(["show", "屯"])
        self.assertEqual(code, 0)
        self.assertIn("初九", out)
        self.assertIn("上六", out)
        code, out = run(["show", "乾"])
        self.assertIn("用九", out)
        code, _ = run(["show", "不存在的卦"])
        self.assertEqual(code, 1)

    def test_bad_config(self):
        self.assertEqual(run(["ask", "q", "--fuzz", "5", "--no-save"])[0], 2)

    def test_options_before_question(self):
        code, out = run(["--seed", "9", "--no-save", "问题"])
        self.assertEqual(code, 0)
        self.assertIn("种子：9", out)

    def test_corrupt_history_line_is_skipped(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "h.jsonl")
            run(["ask", "好的", "--seed", "1", "--history", path])
            with open(path, "a", encoding="utf-8") as f:
                f.write("{坏掉的行\n")
            code, out = run(["history", "--history", path])
            self.assertEqual(code, 0)
            self.assertIn("好的", out)


if __name__ == "__main__":
    unittest.main()
