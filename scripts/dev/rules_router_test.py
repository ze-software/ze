#!/usr/bin/env python3
"""Tests for the trigger-routing coverage report.

Run: python3 scripts/dev/rules_router_test.py

Picked up automatically by `TestPythonUnitTests` (scripts/dev/python_tests_test.go),
which globs `*_test.py` under every root in `pythonTestRoots`.

Each test builds its own rule corpus and its own task corpus, so the live
97-rule set can change without turning these red. The one test that reads the
real tree says so in its name.
"""

import contextlib
import io
import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import rules_condensed
import rules_router

ROOT = Path(__file__).resolve().parents[2]


def write_rule(rules_dir, name, trigger, severity):
    (rules_dir / name).write_text(
        f"# {name}\n**When:** {trigger}\n**Severity:** {severity}\n\n## Directives\n- do it\n",
        encoding="utf-8",
    )


def write_ladder(rules_dir, rung1="never-destroy-work", rung2="rfc-compliance"):
    """The precedence ladder, plus the two rules it names.

    Every fixture needs one: the always-on core is DERIVED from this table, so a
    rule set without it has no core to derive and `core_members` refuses
    (`rules_condensed.LadderError`). The two rules named here are deliberately
    NOT the ones each test asserts on, so they land in the core and leave the
    rules under test routed.
    """
    write_rule(rules_dir, f"{rung1}.md", "before deleting work", "blocking")
    write_rule(rules_dir, f"{rung2}.md", "when writing protocol code", "blocking")
    (rules_dir / "rule-precedence.md").write_text(
        "# Rule Precedence\n**When:** when two rules disagree\n"
        "**Severity:** blocking\n\n## Directives\n\n"
        "| Rung | Governs | Rules | What |\n|---|---|---|---|\n"
        f"| 1 | Irreversible | `{rung1}` | STOP |\n"
        f"| 2 | Outside-facing | `{rung2}` | Implement |\n",
        encoding="utf-8",
    )


def corpus_dir(tasks):
    """A throwaway plan/learned-shaped corpus: one Context section per file."""
    td = tempfile.mkdtemp()
    for i, text in enumerate(tasks):
        Path(td, f"{100 + i}-task.md").write_text(
            f"# {100 + i} -- task\n\n## Context\n\n{text}\n\n## Decisions\n\n- none\n",
            encoding="utf-8",
        )
    return Path(td)


class ReportTest(unittest.TestCase):
    def _rules(self, rules_dir):
        write_ladder(rules_dir)
        write_rule(
            rules_dir,
            "wire-encoding.md",
            "writing or reviewing any wire-encoding path",
            "blocking",
        )
        write_rule(
            rules_dir,
            "appliance-images.md",
            "when a dependabot alert fires on a gokrazy appliance modcache manifest",
            "blocking",
        )
        write_rule(
            rules_dir,
            "colour-choice.md",
            "adding terminal colors or palette styling to a dashboard",
            "advisory",
        )
        return rules_condensed.load_rules(rules_dir)

    def test_report_names_missed_blocking_rule(self):
        """AC-6: a blocking rule no task surfaces is named; advisory is not."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            rules = self._rules(rules_dir)
            tasks = [
                (
                    "The wire encoding path allocated a buffer per UPDATE. Reviewing the "
                    "encoding path showed the writer allocated on every call."
                ),
                "A second task, also about the wire encoding path and its buffer writer.",
            ]
            report = rules_router.build_report(
                rules, rules_router.load_corpus(corpus_dir(tasks))
            )

            self.assertIn("wire-encoding.md", report["surfaced-any"])
            # Blocking, and no task in the corpus surfaces it.
            self.assertIn("appliance-images.md", report["missed-blocking"])
            # Advisory rules are never reported as misses: the signal must stay
            # actionable, and an advisory miss is not worth an operator's turn.
            self.assertNotIn("colour-choice.md", report["missed-blocking"])
            self.assertNotIn("wire-encoding.md", report["missed-blocking"])

    def test_report_lists_surfaced_rules_per_task(self):
        """AC-6, first half: the report says what each task would surface."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            rules = self._rules(rules_dir)
            tasks = ["The wire encoding path allocated a buffer on the encoding path."]
            report = rules_router.build_report(
                rules, rules_router.load_corpus(corpus_dir(tasks))
            )

            self.assertEqual(len(report["tasks"]), 1)
            self.assertIn("wire-encoding.md", report["tasks"][0]["surfaced"])
            self.assertEqual(
                report["tasks"][0]["surfaced"].count("wire-encoding.md"), 1
            )

    def test_empty_corpus_reports_every_blocking_rule_missed(self):
        """Fail closed: no evidence means no coverage claimed, never the reverse."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            rules = self._rules(rules_dir)
            report = rules_router.build_report(rules, [])
            self.assertIn("wire-encoding.md", report["missed-blocking"])
            self.assertIn("appliance-images.md", report["missed-blocking"])

    def test_core_rules_are_not_counted_as_missed(self):
        """A rule already eager cannot be missed by routing: it is never routed."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_rule(
                rules_dir, "never-destroy-work.md", "before deleting work", "blocking"
            )
            write_rule(
                rules_dir, "rfc-compliance.md", "when writing protocol code", "blocking"
            )
            (rules_dir / "rule-precedence.md").write_text(
                "# Rule Precedence\n**When:** when two rules disagree\n"
                "**Severity:** blocking\n\n## Directives\n\n"
                "| Rung | Governs | Rules | What |\n|---|---|---|---|\n"
                "| 1 | Irreversible | `never-destroy-work` | STOP |\n"
                "| 2 | Outside-facing | `rfc-compliance` | Implement |\n",
                encoding="utf-8",
            )
            rules = rules_condensed.load_rules(rules_dir)
            report = rules_router.build_report(rules, [])
            self.assertEqual(report["missed-blocking"], [])
            self.assertEqual(sorted(report["core"]), sorted(r["name"] for r in rules))

    def test_report_refuses_an_unreadable_ladder(self):
        """The core this report subtracts is derived, so an empty parse refuses.

        Every number the report prints is computed after the core is removed
        from the routed set. A ladder that parsed to nothing would silently move
        four guards into `missed-blocking` and read as a routing gap.
        """
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            self._rules(rules_dir)
            path = rules_dir / "rule-precedence.md"
            path.write_text(
                path.read_text(encoding="utf-8").replace("| Rung |", "| Level |"),
                encoding="utf-8",
            )
            rules = rules_condensed.load_rules(rules_dir)
            with self.assertRaises(rules_condensed.LadderError):
                rules_router.build_report(rules, [])

    def test_main_exits_non_zero_on_an_unreadable_ladder(self):
        """The refusal reaches the exit code, so a wrong report is never printed."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            self._rules(rules_dir)
            path = rules_dir / "rule-precedence.md"
            path.write_text(
                path.read_text(encoding="utf-8").replace("| Rung |", "| Level |"),
                encoding="utf-8",
            )
            err = io.StringIO()
            with (
                contextlib.redirect_stdout(io.StringIO()),
                contextlib.redirect_stderr(err),
            ):
                code = rules_router.main(["--rules-dir", str(rules_dir)])
            self.assertEqual(code, 1)
            self.assertIn("rule-precedence.md", err.getvalue())

    def test_main_exits_zero_on_a_readable_ladder(self):
        """Without this, the test above would pass for any failure at all."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            self._rules(rules_dir)
            with contextlib.redirect_stdout(io.StringIO()):
                code = rules_router.main(["--rules-dir", str(rules_dir)])
            self.assertEqual(code, 0)

    def test_live_corpus_is_not_empty(self):
        """The live tree: the report has real tasks to measure against.

        The corpus is `plan/spec-*.md` alone since the learned corpus became
        `plan/journal/` (plan/spec-problem-journal.md): a journal row records a
        problem class, not a task description.
        """
        corpus = rules_router.load_corpus(ROOT / "plan")
        self.assertGreater(len(corpus), 50)
        for task in corpus[:5]:
            self.assertTrue(task["text"].strip())


if __name__ == "__main__":
    unittest.main()
