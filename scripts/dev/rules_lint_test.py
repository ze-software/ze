#!/usr/bin/env python3
"""Tests for the RFC 2119 pass of the rule linter.

Run: python3 scripts/dev/rules_lint_test.py

Picked up automatically by `TestPythonUnitTests` (scripts/dev/python_tests_test.go),
which globs `*_test.py` under every root in `pythonTestRoots`.

Every test builds its own point file, so the live corpus can change without
turning these red.
"""

import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import rules_lint


def point(body, kind="directive", level=""):
    return f"---\nkind: {kind}\nlevel: {level}\nstage:\n---\n{body}\n"


def check(body, **kw):
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / "a-slug.md"
        text = point(body, **kw)
        path.write_text(text)
        return rules_lint.check_point(path, text)


class TestDirectiveStatesALevel(unittest.TestCase):
    def test_a_directive_without_a_keyword_is_refused(self):
        problems = check("- Delete the old path before writing the new one.")
        self.assertTrue(any("RFC 2119 language" in p for p in problems), problems)

    def test_a_directive_with_a_keyword_and_a_matching_level_passes(self):
        self.assertEqual(
            check("- You MUST delete the old path first.", level="MUST"), []
        )

    def test_a_note_is_not_asked_for_a_keyword(self):
        self.assertEqual(check("Background prose, no obligation.", kind="note"), [])

    def test_a_table_is_not_asked_for_a_keyword(self):
        self.assertEqual(check("| Path | Holds |\n|---|---|", kind="table"), [])


class TestLevelAgreesWithTheBody(unittest.TestCase):
    def test_an_empty_level_beside_a_stated_one_is_refused(self):
        problems = check("- You MUST do it.")
        self.assertTrue(any("disagrees with the body" in p for p in problems), problems)

    def test_the_strongest_tier_wins(self):
        self.assertEqual(
            check("- You MAY skip it, but you MUST say so.", level="MUST"), []
        )

    def test_the_must_tier_outranks_the_should_tier(self):
        self.assertEqual(
            check("- You SHOULD ask, and you MUST NOT guess.", level="MUST NOT"), []
        )

    def test_a_weaker_tier_than_the_body_states_is_refused(self):
        problems = check("- You SHOULD ask, and you MUST NOT guess.", level="SHOULD")
        self.assertTrue(any("disagrees with the body" in p for p in problems), problems)

    def test_either_polarity_of_the_stated_tier_is_accepted(self):
        # RFC 2119 does not rank MUST against MUST NOT. A point stating both is
        # free to declare the one its central clause carries.
        body = "- You MUST record the finding, and you MUST NOT park it."
        self.assertEqual(check(body, level="MUST"), [])
        self.assertEqual(check(body, level="MUST NOT"), [])

    def test_a_synonym_collapses_onto_the_level_it_names(self):
        self.assertEqual(check("- A tagged test is REQUIRED.", level="MUST"), [])

    def test_an_unknown_level_is_refused(self):
        problems = check("- You MUST do it.", level="OBLIGATORY")
        self.assertTrue(any("is not one of" in p for p in problems), problems)


class TestLowercaseModals(unittest.TestCase):
    def test_a_lowercase_modal_is_refused(self):
        problems = check("- You MUST act, and you should also report.", level="MUST")
        self.assertTrue(any("lowercase obligation" in p for p in problems), problems)

    def test_a_modal_inside_a_code_span_is_quoted_not_stated(self):
        self.assertEqual(
            check("- The error MUST read `the file must exist`.", level="MUST"), []
        )

    def test_a_modal_inside_a_fenced_block_is_quoted_not_stated(self):
        body = (
            "- You MUST use the flag below.\n\n```\nfoo --may-fail\nbar must run\n```"
        )
        self.assertEqual(check(body, level="MUST"), [])

    def test_a_modal_inside_a_tilde_fence_is_quoted_not_stated(self):
        body = "- You MUST preserve this example.\n\n~~~~\nbar should run\n~~~~~"
        self.assertEqual(check(body, level="MUST"), [])

    def test_a_modal_inside_a_blockquote_is_quoted_not_stated(self):
        body = "- You MUST preserve this quotation.\n\n> The agent should ask."
        self.assertEqual(check(body, level="MUST"), [])

    def test_a_keyword_only_inside_a_tilde_fence_does_not_set_the_level(self):
        problems = check("- The example follows.\n\n~~~~\nbar MUST run\n~~~~~")
        self.assertTrue(any("RFC 2119 language" in p for p in problems), problems)

    def test_a_keyword_only_inside_a_blockquote_does_not_set_the_level(self):
        problems = check("- The quotation follows.\n\n> The agent MUST ask.")
        self.assertTrue(any("RFC 2119 language" in p for p in problems), problems)


    def test_a_hyphenated_word_is_not_a_modal(self):
        self.assertEqual(check("- A must-fix defect MUST be fixed.", level="MUST"), [])


class TestFrontmatter(unittest.TestCase):
    def test_a_point_without_frontmatter_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "a-slug.md"
            text = "- No frontmatter here.\n"
            path.write_text(text)
            problems = rules_lint.check_point(path, text)
        self.assertTrue(any("frontmatter" in p for p in problems), problems)


class TestTheLiveCorpus(unittest.TestCase):
    def test_every_directive_in_the_repo_states_a_level(self):
        points = Path(__file__).resolve().parents[2] / "ai" / "rules" / "points"
        if not points.is_dir():
            self.skipTest("ai/rules/points/ not present")
        _, failures = rules_lint.check_points(points)
        # The NAMES only: a corpus-wide diff of every violation body is
        # unreadable, and the linter prints the detail on demand.
        self.assertEqual(
            sorted(failures),
            [],
            "run: python3 scripts/dev/rules_lint.py --points",
        )


if __name__ == "__main__":
    unittest.main()
