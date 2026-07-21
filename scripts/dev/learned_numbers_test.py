#!/usr/bin/env python3
"""Unit tests for learned_numbers.py (plan/learned numbering invariants)."""

from __future__ import annotations

import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from learned_numbers import (
    check,
    duplicates,
    h1_number,
    rename_plan,
    retitle,
    summaries,
)


def learned_dir(tmp: str, files: dict[str, str]) -> Path:
    d = Path(tmp) / "plan" / "learned"
    d.mkdir(parents=True)
    for name, text in files.items():
        (d / name).write_text(text)
    return d


class TestSummaries(unittest.TestCase):
    def test_groups_by_number_and_ignores_unnumbered(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = learned_dir(
                tmp,
                {
                    "001-foo.md": "# 001 -- Foo\n",
                    "001-bar.md": "# 001 -- Bar\n",
                    "002-baz.md": "# 002 -- Baz\n",
                    "README.md": "# not numbered\n",
                },
            )
            self.assertEqual(
                summaries(d), {1: ["001-bar.md", "001-foo.md"], 2: ["002-baz.md"]}
            )

    def test_duplicates_reports_only_collisions(self):
        items = {1: ["001-a.md", "001-b.md"], 2: ["002-c.md"]}
        self.assertEqual(duplicates(items), {1: ["001-a.md", "001-b.md"]})


class TestH1(unittest.TestCase):
    def test_reads_each_separator_form(self):
        self.assertEqual(h1_number("# 477 -- DNS Resolver\n"), 477)
        self.assertEqual(h1_number("# 821 — Plugin keyword\n"), 821)
        self.assertEqual(h1_number("# 610: bng-2 -- Counters\n"), 610)

    def test_none_when_heading_carries_no_number(self):
        self.assertIsNone(h1_number("# Learned: RIB extraction\n"))
        self.assertIsNone(h1_number("no heading at all\n"))

    def test_retitle_preserves_separator(self):
        self.assertEqual(
            retitle("# 477 -- DNS\n\nbody\n", 1129), "# 1129 -- DNS\n\nbody\n"
        )
        self.assertEqual(retitle("# 610: bng-2 -- X\n", 1137), "# 1137: bng-2 -- X\n")
        self.assertEqual(retitle("# 821 — P\n", 1145), "# 1145 — P\n")

    def test_retitle_leaves_unnumbered_heading_alone(self):
        self.assertEqual(retitle("# Learned: X\n", 5), "# Learned: X\n")


class TestCheck(unittest.TestCase):
    def test_clean_tree_has_no_problems(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = learned_dir(
                tmp,
                {"001-foo.md": "# 001 -- Foo\n", "002-bar.md": "# 002 -- Bar\n"},
            )
            self.assertEqual(check(d), [])

    def test_duplicate_number_is_reported(self):
        # The regression this tool exists for: two branches allocate the same
        # number and it only surfaces on merge.
        with tempfile.TemporaryDirectory() as tmp:
            d = learned_dir(
                tmp,
                {"007-vrrp.md": "# 007 -- VRRP\n", "007-rib.md": "# 007 -- RIB\n"},
            )
            problems = check(d)
            self.assertEqual(len(problems), 1)
            self.assertIn("number 7 claimed by 2 summaries", problems[0])
            self.assertIn("007-rib.md", problems[0])
            self.assertIn("007-vrrp.md", problems[0])

    def test_h1_filename_mismatch_is_reported(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = learned_dir(tmp, {"409-gc.md": "# 401 -- GC Pressure\n"})
            problems = check(d)
            self.assertEqual(len(problems), 1)
            self.assertIn("H1 says 401, filename says 409", problems[0])

    def test_check_passes_without_counter(self):
        # AC-7/R-7: .counter is retired. A tree with no .counter is not a
        # problem, and invariants 1 (uniqueness) and 2 (H1 matches filename) --
        # the reason this tool exists -- still fire on seeded breakage.
        with tempfile.TemporaryDirectory() as tmp:
            clean = learned_dir(
                tmp,
                {"100-foo.md": "# 100 -- Foo\n", "101-bar.md": "# 101 -- Bar\n"},
            )
            self.assertFalse((clean / ".counter").exists())
            self.assertEqual(check(clean), [])

        with tempfile.TemporaryDirectory() as tmp:
            dup = learned_dir(
                tmp,
                {"200-a.md": "# 200 -- A\n", "200-b.md": "# 200 -- B\n"},
            )
            problems = check(dup)
            self.assertEqual(len(problems), 1)
            self.assertIn("number 200 claimed by 2 summaries", problems[0])

        with tempfile.TemporaryDirectory() as tmp:
            mismatch = learned_dir(tmp, {"300-x.md": "# 299 -- X\n"})
            problems = check(mismatch)
            self.assertEqual(len(problems), 1)
            self.assertIn("H1 says 299, filename says 300", problems[0])

    def test_unnumbered_heading_is_not_a_mismatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = learned_dir(tmp, {"100-foo.md": "# Learned: Foo\n"})
            self.assertEqual(check(d), [])


class TestRenamePlan(unittest.TestCase):
    def test_no_duplicates_yields_empty_plan(self):
        items = {1: ["001-a.md"], 2: ["002-b.md"]}
        self.assertEqual(rename_plan(items, {}, {}), [])

    def test_most_referenced_keeps_the_number(self):
        items = {7: ["007-rib.md", "007-vrrp.md"]}
        refs = {"007-vrrp.md": 43, "007-rib.md": 1}
        plan = rename_plan(items, refs, {"007-vrrp.md": 1, "007-rib.md": 2})
        self.assertEqual(plan, [("007-rib.md", "8-rib.md", 8)])

    def test_tie_on_refs_breaks_by_earliest_add(self):
        items = {7: ["007-late.md", "007-early.md"]}
        refs = {"007-late.md": 0, "007-early.md": 0}
        added = {"007-early.md": 100, "007-late.md": 200}
        plan = rename_plan(items, refs, added)
        self.assertEqual(plan, [("007-late.md", "8-late.md", 8)])

    def test_numbers_are_assigned_above_the_highest_existing(self):
        items = {1: ["001-a.md", "001-b.md"], 50: ["050-c.md"]}
        plan = rename_plan(items, {"001-a.md": 5}, {})
        self.assertEqual(plan, [("001-b.md", "51-b.md", 51)])

    def test_multiple_groups_get_sequential_fresh_numbers(self):
        items = {
            1: ["001-a.md", "001-b.md"],
            2: ["002-c.md", "002-d.md", "002-e.md"],
        }
        refs = {"001-a.md": 9, "002-c.md": 9}
        plan = rename_plan(items, refs, {})
        self.assertEqual(
            plan,
            [
                ("001-b.md", "3-b.md", 3),
                ("002-d.md", "4-d.md", 4),
                ("002-e.md", "5-e.md", 5),
            ],
        )

    def test_deterministic(self):
        items = {1: ["001-a.md", "001-b.md", "001-c.md"]}
        self.assertEqual(rename_plan(items, {}, {}), rename_plan(items, {}, {}))


if __name__ == "__main__":
    unittest.main()
