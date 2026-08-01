#!/usr/bin/env python3
"""Unit tests for learned_normalise.py (plan/learned section-heading normaliser)."""

from __future__ import annotations

import contextlib
import io
import os
import stat
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from learned_normalise import normalise, problems, run

CONFORMING = """# 900 -- Already Fine

## Context

Something happened.

## Decisions

- chose A over B

## Consequences

- nothing

## Gotchas

- none

## Files

- `scripts/dev/thing.py`
"""


class TestHeadingMigration(unittest.TestCase):
    def test_objective_becomes_context(self):
        # VALIDATES: AC-8 -- no `## Objective` heading remains
        text = "# 12 -- T\n\n## Objective\n\nBody.\n\n## Files\n\n- `a.go`\n"
        out = normalise(text)
        self.assertIn("## Context\n", out)
        self.assertNotIn("## Objective", out)

    def test_h1_is_untouched(self):
        # PREVENTS: breaking learned_numbers.py's H1-versus-filename invariant
        text = "# 12 -- Objective Lens\n\n## Objective\n\nB.\n\n## Files\n\n- `a`\n"
        self.assertIn("# 12 -- Objective Lens\n", normalise(text))

    def test_word_in_prose_is_untouched(self):
        # PREVENTS: rewriting a line that merely contains the word
        text = (
            "# 12 -- T\n\n## Context\n\nThe ## Objective heading was retired.\n"
            "Our objective was speed.\n\n## Files\n\n- `a`\n"
        )
        self.assertEqual(normalise(text), text)

    def test_deeper_heading_is_untouched(self):
        text = "# 12 -- T\n\n### Objective\n\nB.\n\n## Files\n\n- `a`\n"
        self.assertIn("### Objective", normalise(text))

    def test_collision_is_reported_not_renamed(self):
        # A rename here would leave two `## Context` headings. Fail closed.
        text = (
            "# 12 -- T\n\n## Context\n\nA.\n\n## Objective\n\nB.\n\n## Files\n\n- `a`\n"
        )
        self.assertEqual(normalise(text), text)
        self.assertTrue(any("cannot be renamed" in p for p in problems(text)))


class TestFilesSection(unittest.TestCase):
    def test_missing_files_section_is_appended(self):
        # VALIDATES: AC-8 -- every summary has a `## Files` section
        text = "# 12 -- T\n\n## Context\n\nBody.\n"
        out = normalise(text)
        self.assertTrue(out.endswith("## Files\n\nNone recorded.\n"))
        self.assertIn("## Context\n\nBody.\n", out)

    def test_appended_section_is_separated_by_a_blank_line(self):
        out = normalise("# 12 -- T\n\n## Context\n\nBody.\n\n\n\n")
        self.assertIn("Body.\n\n## Files\n", out)

    def test_files_variant_is_canonicalised_not_fabricated(self):
        # `## Files Changed` records real paths. Appending "None recorded."
        # would state something false and hide those paths from the later gate.
        text = "# 12 -- T\n\n## Context\n\nB.\n\n## Files Changed\n\n- `a.go`\n"
        out = normalise(text)
        self.assertIn("## Files\n\n- `a.go`\n", out)
        self.assertNotIn("None recorded.", out)

    def test_only_the_first_files_variant_is_canonicalised(self):
        text = (
            "# 12 -- T\n\n## Context\n\nB.\n\n## Files Created\n\n- `a`\n"
            "\n## Files Modified\n\n- `b`\n"
        )
        out = normalise(text)
        self.assertIn("## Files\n\n- `a`\n", out)
        self.assertIn("## Files Modified\n", out)

    def test_filesystem_heading_is_not_a_files_heading(self):
        text = "# 12 -- T\n\n## Context\n\nB.\n\n## Filesystem notes\n\n- `a`\n"
        out = normalise(text)
        self.assertIn("## Filesystem notes", out)
        self.assertTrue(out.endswith("## Files\n\nNone recorded.\n"))


class TestIdempotency(unittest.TestCase):
    def test_conforming_summary_is_untouched(self):
        self.assertEqual(normalise(CONFORMING), CONFORMING)
        self.assertEqual(problems(CONFORMING), [])

    def test_running_twice_changes_nothing(self):
        for text in (
            "# 12 -- T\n\n## Objective\n\nB.\n",
            "# 12 -- T\n\n## Context\n\nB.\n",
            "# 12 -- T\n\n## Context\n\nB.\n\n## Files Touched\n\n- `a`\n",
        ):
            once = normalise(text)
            self.assertEqual(normalise(once), once, msg=text)
            self.assertEqual(problems(once), [], msg=text)


class TreeCase(unittest.TestCase):
    """Builds a throwaway `plan/learned/`. Carries no test of its own, so a
    subclass adds cases rather than re-running someone else's."""

    def _tree(self, tmp: str, files: dict[str, str]) -> Path:
        learned = Path(tmp) / "plan" / "learned"
        learned.mkdir(parents=True)
        for name, body in files.items():
            (learned / name).write_text(body, encoding="utf-8")
        return learned


class TestRun(TreeCase):
    def test_check_exits_one_and_fix_exits_zero(self):
        with tempfile.TemporaryDirectory() as tmp:
            learned = self._tree(
                tmp,
                {
                    "001-a.md": "# 1 -- A\n\n## Objective\n\nB.\n",
                    "900-b.md": CONFORMING,
                    "README.md": "# not numbered\n",  # ignored: not [0-9]*.md
                },
            )
            self.assertEqual(run(learned, fix=False), 1)
            self.assertEqual(run(learned, fix=True), 0)
            self.assertEqual(run(learned, fix=False), 0)

            fixed = (learned / "001-a.md").read_text(encoding="utf-8")
            self.assertIn("## Context", fixed)
            self.assertTrue(fixed.endswith("## Files\n\nNone recorded.\n"))
            self.assertEqual(
                (learned / "900-b.md").read_text(encoding="utf-8"), CONFORMING
            )
            self.assertEqual(
                (learned / "README.md").read_text(encoding="utf-8"), "# not numbered\n"
            )

    def test_empty_directory_is_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(run(self._tree(tmp, {}), fix=False), 1)


class TestFileMode(TreeCase):
    """VALIDATES: a rewrite leaves the summary's mode exactly as it found it.
    PREVENTS: `tempfile.mkstemp` narrowing the corpus from 0644 to 0600. Git
    tracks only the exec bit, so that narrowing never shows in `git diff`; it
    reached 228 summaries once before anyone saw it."""

    def _rewrite_with_mode(self, mode: int) -> int:
        with tempfile.TemporaryDirectory() as tmp:
            learned = self._tree(tmp, {"001-a.md": "# 1 -- A\n\n## Objective\n\nB.\n"})
            target = learned / "001-a.md"
            os.chmod(target, mode)

            self.assertEqual(run(learned, fix=True), 0)
            self.assertIn("## Context", target.read_text(encoding="utf-8"))
            return stat.S_IMODE(target.stat().st_mode)

    def test_the_corpus_baseline_mode_survives(self):
        self.assertEqual(self._rewrite_with_mode(0o644), 0o644)

    def test_a_non_baseline_mode_is_carried_across_unchanged(self):
        # Preserved, not reset to a hardcoded 0644.
        self.assertEqual(self._rewrite_with_mode(0o640), 0o640)


class TestUnactionedIssuesAreNeverSilent(TreeCase):
    """VALIDATES: an issue normalise() declines is reported in BOTH branches.
    PREVENTS: a refusal swallowed by a change that succeeded in the same file.
    `## Objective` + `## Context` + no `## Files` used to report the collision
    nowhere, count no blocker and exit 0, because appending `## Files` made
    `new != text` and took the changed branch."""

    COLLIDING = "# 12 -- T\n\n## Context\n\nA.\n\n## Objective\n\nB.\n"

    def _run(self, fix: bool) -> tuple[int, str, str, str]:
        with tempfile.TemporaryDirectory() as tmp:
            learned = self._tree(tmp, {"001-a.md": self.COLLIDING})
            out, err = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                code = run(learned, fix=fix)
            body = (learned / "001-a.md").read_text(encoding="utf-8")
            return code, out.getvalue(), err.getvalue(), body

    def test_fix_reports_the_collision_and_still_exits_nonzero(self):
        code, _, err, body = self._run(fix=True)
        self.assertEqual(code, 1, "an un-actioned refusal must not exit 0")
        self.assertIn("cannot be renamed", err)
        self.assertIn("need a human decision", err)
        # The actionable half still happened; only the refusal is withheld.
        self.assertIn("## Files", body)
        self.assertIn("## Objective", body)

    def test_check_reports_the_refusal_and_the_repair_separately(self):
        code, out, err, _ = self._run(fix=False)
        self.assertEqual(code, 1)
        self.assertIn("cannot be renamed", err)
        self.assertIn("no `## Files` section", out)
        # The refusal is not also announced as a repair that will happen.
        self.assertNotIn("cannot be renamed", out)


class TestDuplicateObjectiveIsRefused(TreeCase):
    """VALIDATES: two `## Objective` headings are refused, not both renamed.
    PREVENTS: normalise() manufacturing the duplicate `## Context` that the
    collision guard three lines above it exists to prevent."""

    TWO = "# 12 -- T\n\n## Objective\n\nA.\n\n## Objective\n\nB.\n\n## Files\n\n- `a`\n"

    def test_neither_heading_is_renamed(self):
        out = normalise(self.TWO)
        self.assertEqual(out, self.TWO)
        self.assertEqual(out.count("## Context"), 0, "a duplicate was manufactured")

    def test_the_refusal_names_the_reason(self):
        found = problems(self.TWO)
        self.assertTrue(any("would be duplicated" in p for p in found), found)

    def test_run_counts_it_as_blocked_and_exits_nonzero(self):
        with tempfile.TemporaryDirectory() as tmp:
            learned = self._tree(tmp, {"001-a.md": self.TWO})
            err = io.StringIO()
            with (
                contextlib.redirect_stdout(io.StringIO()),
                contextlib.redirect_stderr(err),
            ):
                code = run(learned, fix=True)
            self.assertEqual(code, 1)
            self.assertIn("would be duplicated", err.getvalue())
            self.assertEqual(
                (learned / "001-a.md").read_text(encoding="utf-8"), self.TWO
            )

    def test_a_single_objective_is_still_renamed(self):
        # The guard is on the count, not on the heading.
        one = "# 12 -- T\n\n## Objective\n\nA.\n\n## Files\n\n- `a`\n"
        self.assertIn("## Context", normalise(one))


if __name__ == "__main__":
    unittest.main()
