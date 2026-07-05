#!/usr/bin/env python3
"""Unit tests for learned_index.py (ai/LEARNED-FULL-INDEX.md generator)."""

from __future__ import annotations

import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from learned_index import entries, render


class TestLearnedIndex(unittest.TestCase):
    def test_numbers_titles_and_grouping(self):
        with tempfile.TemporaryDirectory() as tmp:
            learned = Path(tmp) / "plan" / "learned"
            learned.mkdir(parents=True)
            (learned / "001-foo.md").write_text("# 001 -- Foo Bar\n\ntext\n")
            (learned / "100-baz.md").write_text("# 100 — Baz Qux\n")
            (learned / ".counter").write_text("101\n")  # ignored: not [0-9]*.md
            (learned / "README.md").write_text("# not numbered\n")  # ignored

            items = entries(learned)
            self.assertEqual(
                items,
                [(1, "Foo Bar", "001-foo.md"), (100, "Baz Qux", "100-baz.md")],
            )

            content = render(items)
            self.assertIn("## 000-099", content)
            self.assertIn("## 100-199", content)
            self.assertIn("Total: 2 summaries", content)

    def test_falls_back_to_slug_without_title(self):
        with tempfile.TemporaryDirectory() as tmp:
            learned = Path(tmp) / "plan" / "learned"
            learned.mkdir(parents=True)
            (learned / "007-no-title.md").write_text("no heading here\n")
            self.assertEqual(entries(learned), [(7, "007-no-title", "007-no-title.md")])

    def test_deterministic(self):
        with tempfile.TemporaryDirectory() as tmp:
            learned = Path(tmp) / "plan" / "learned"
            learned.mkdir(parents=True)
            (learned / "002-a.md").write_text("# 002 -- A\n")
            (learned / "003-b.md").write_text("# 003 -- B\n")
            self.assertEqual(render(entries(learned)), render(entries(learned)))


if __name__ == "__main__":
    unittest.main()
