#!/usr/bin/env python3
"""Tests for the rule-coverage miss-detector.

Run: python3 scripts/dev/rule_coverage_test.py

Picked up automatically by `TestPythonUnitTests` (scripts/dev/python_tests_test.go),
which globs `*_test.py` under every root in `pythonTestRoots`.

Each test builds its own tiny rule corpus and its own transcript, so nothing
here depends on the live 97-rule set: a rule edited elsewhere must never turn
these red.
"""

import contextlib
import io
import json
import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import rule_coverage
import rules_condensed

HOOK = (
    Path(__file__).resolve().parents[2]
    / ".claude"
    / "hooks"
    / "rule-coverage-report.sh"
)


def write_rule(rules_dir, name, trigger, severity):
    (rules_dir / name).write_text(
        f"# {name}\n**When:** {trigger}\n**Severity:** {severity}\n\n## Directives\n- do it\n",
        encoding="utf-8",
    )


def write_transcript(path, calls):
    """calls: list of (tool_name, file_path)."""
    with open(path, "w", encoding="utf-8") as fh:
        fh.writelines(
            json.dumps(
                {
                    "type": "assistant",
                    "message": {
                        "role": "assistant",
                        "content": [
                            {
                                "type": "tool_use",
                                "name": tool,
                                "input": {"file_path": fp},
                            }
                        ],
                    },
                }
            )
            + "\n"
            for tool, fp in calls
        )


class DetectorCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.rules = self.root / "ai" / "rules"
        self.rules.mkdir(parents=True)
        self.transcript = self.root / "session.jsonl"
        # repo_root() reads CLAUDE_PROJECT_DIR, so rel_path() resolves the
        # fabricated absolute paths below against this sandbox.
        self._prev = os.environ.get("CLAUDE_PROJECT_DIR")
        os.environ["CLAUDE_PROJECT_DIR"] = str(self.root)

    def tearDown(self):
        if self._prev is None:
            os.environ.pop("CLAUDE_PROJECT_DIR", None)
        else:
            os.environ["CLAUDE_PROJECT_DIR"] = self._prev
        self.tmp.cleanup()

    def run_detector(self, extra=()):
        argv = [
            "--transcript",
            str(self.transcript),
            "--rules-dir",
            str(self.rules),
            "--json",
            "--no-append",
            *extra,
        ]
        return rule_coverage.main(argv)

    def analyse(self):
        rules = rule_coverage.load_rules(self.rules)
        written, read = rule_coverage.read_transcript(str(self.transcript))
        return rule_coverage.analyse(rules, written, read)


class TestDetector(DetectorCase):
    def test_detector_reports_unread_matched_rule(self):
        """AC-10: a blocking rule whose trigger matched, never read, is named."""
        write_rule(
            self.rules, "performance.md", "writing any wire-encoding path", "blocking"
        )
        write_transcript(
            self.transcript,
            [("Edit", str(self.root / "internal" / "component" / "bgp" / "wire.go"))],
        )

        result = self.analyse()

        self.assertIn("performance.md", result["missed"])
        self.assertEqual(1, self.run_detector(), "a miss must exit 1, not 0")

    def test_detector_silent_when_all_read(self):
        """AC-11: reading the matched rule clears it."""
        write_rule(
            self.rules, "performance.md", "writing any wire-encoding path", "blocking"
        )
        write_transcript(
            self.transcript,
            [
                ("Edit", str(self.root / "internal" / "wire.go")),
                ("Read", str(self.root / "ai" / "rules" / "performance.md")),
            ],
        )

        result = self.analyse()

        self.assertIn("performance.md", result["matched"])
        self.assertEqual([], result["missed"])
        self.assertEqual(0, self.run_detector())

    def test_detector_ignores_advisory_rules(self):
        """AC-11 / signal quality: only `blocking` rules are ever reported."""
        write_rule(
            self.rules, "blocking-one.md", "writing any wire-encoding path", "blocking"
        )
        write_rule(
            self.rules, "advisory-one.md", "writing any wire-encoding path", "advisory"
        )
        write_transcript(
            self.transcript, [("Edit", str(self.root / "internal" / "wire.go"))]
        )

        result = self.analyse()

        self.assertEqual(["blocking-one.md"], result["missed"])
        self.assertNotIn("advisory-one.md", result["matched"])
        self.assertEqual(1, result["blocking-total"])

    def test_condensed_digest_read_does_not_count(self):
        """Reading the eager digest must not mark every rule consulted.

        This is what makes a miss meaningful: a digest is in every session's
        context today, so counting it would make the detector measure nothing.
        """
        write_rule(
            self.rules, "performance.md", "writing any wire-encoding path", "blocking"
        )
        (self.rules / "TRIGGERS.md").write_text("# digest\n", encoding="utf-8")
        write_transcript(
            self.transcript,
            [
                ("Edit", str(self.root / "internal" / "wire.go")),
                ("Read", str(self.rules / "TRIGGERS.md")),
            ],
        )

        result = self.analyse()

        self.assertEqual(["performance.md"], result["missed"])

    def test_point_read_does_not_count_as_reading_its_rule(self):
        """Reading one point of a rule is not reading the rule.

        VALIDATES: `_is_rule_path` accepts only a file sitting DIRECTLY in
        ai/rules/, so a Read of `ai/rules/points/<rule>/<section>/<slug>.md`
        credits nothing.
        PREVENTS: the false clear that a basename match produces. Slugs are
        derived from the block's first line, and four of the real corpus's 2316
        slugs already equal a rule stem (`architecture`, `completion`,
        `git-safety`, `testing`). Under a bare prefix test, opening
        `ai/rules/points/plugins/directives/architecture.md` would mark the blocking rule <!-- doc-links: ignore (constructed example: a point slug equal to a rule stem) -->
        `ai/rules/architecture.md` as consulted.
        """
        write_rule(
            self.rules, "performance.md", "writing any wire-encoding path", "blocking"
        )
        point = self.rules / "points" / "plugins" / "directives" / "performance.md"
        point.parent.mkdir(parents=True)
        point.write_text("---\nkind: note\n---\nbody\n", encoding="utf-8")
        write_transcript(
            self.transcript,
            [
                ("Edit", str(self.root / "internal" / "wire.go")),
                ("Read", str(point)),
            ],
        )

        _, rules_read = rule_coverage.read_transcript(str(self.transcript))

        self.assertEqual(set(), rules_read)
        self.assertEqual(["performance.md"], self.analyse()["missed"])

    def test_rule_read_still_counts_after_the_point_exclusion(self):
        """The exclusion must not mute the rule file itself.

        A guard that refuses everything passes the test above and measures
        nothing (`ai/rules/evidence.md`). This is its other half.
        """
        write_rule(
            self.rules, "performance.md", "writing any wire-encoding path", "blocking"
        )
        (self.rules / "points" / "performance").mkdir(parents=True)
        write_transcript(
            self.transcript,
            [
                ("Edit", str(self.root / "internal" / "wire.go")),
                ("Read", str(self.rules / "performance.md")),
            ],
        )

        _, rules_read = rule_coverage.read_transcript(str(self.transcript))

        self.assertEqual({"performance.md"}, rules_read)
        self.assertEqual([], self.analyse()["missed"])

    def write_core(self, *names, cites="completion.md"):
        """A CORE.md carrying the given rules, in the generator's own shape.

        The directive body CITES another rule inline, exactly as the real
        `ai/rules/CORE.md` does throughout. That citation is the reason
        `CORE_RULE_LINE` is `^`/`$` anchored, and a fixture without one lets the
        anchors be deleted with every test still green. On the live artifact
        that deletion nearly doubles the muted set, swallowing rules every
        session is expected to read (`completion.md`, `testing.md`, `testing.md`,
        `planning.md` among them): over-muting, the one direction this module
        exists to prevent. The exact counts move with the corpus, so they are
        measured rather than written down here.
        """
        body = "# Ze Rules -- Always-On Core\n\n"
        for name in names:
            body += (
                f"## {name}\n"
                f"`ai/rules/{name}`\n"  # <!-- doc-links: ignore (format template, not a path) -->
                "**When:** always\n\n"
                f"Fix the root cause (`ai/rules/{cites}`), never record it.\n\n"  # <!-- doc-links: ignore (format template, not a path) -->
            )
        (self.rules / "CORE.md").write_text(body, encoding="utf-8")

    def test_inline_citation_in_core_is_not_treated_as_membership(self):
        """A rule CITED inside CORE.md is not a rule CARRIED by CORE.md.

        Deleting the `^`/`$` anchors from CORE_RULE_LINE makes this red. Without
        it, that deletion mutes a rule every session is expected to read.
        """
        write_rule(
            self.rules, "spec-no-code.md", "writing or editing a spec", "blocking"
        )
        write_rule(
            self.rules, "completion.md", "when creating or updating a spec", "blocking"
        )
        self.write_core("spec-no-code.md", cites="completion.md")
        write_transcript(
            self.transcript, [("Edit", str(self.root / "plan" / "spec-thing.md"))]
        )

        result = self.analyse()

        self.assertEqual(
            ["spec-no-code.md"],
            result["always-on-rules"],
            "only the standalone path line is membership; an inline citation is prose",
        )
        self.assertEqual(
            ["completion.md"], result["missed"], "the cited rule is still owed"
        )

    def test_parse_matches_the_generator_that_writes_core(self):
        """Pin the reader to `rules_condensed.rule_block`, its actual producer.

        `write_core` above is this file's own idea of the shape. If the
        generator's changes, every other test here stays green while the live
        exclusion silently empties (`ai/rules/evidence.md`).
        """
        sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
        import rules_condensed

        block = rules_condensed.rule_block(
            {
                "path": "ai/rules/spec-no-code.md",
                "title": "No Code in Specs",
                "meta": [
                    ("When", "writing or editing a spec"),
                    ("Severity", "blocking"),
                ],
                "body": ["## Directives", "", "Specs MUST NOT contain code."],
            }
        )
        (self.rules / "CORE.md").write_text(block + "\n", encoding="utf-8")

        self.assertEqual(
            {"spec-no-code.md"},
            rule_coverage.always_on_rules(self.rules),
            "CORE_RULE_LINE no longer matches what rule_block emits",
        )

    def test_live_core_artifact_round_trips_writer_to_reader(self):
        """The real `ai/rules/CORE.md`, not a fixture: writer and reader agree.

        Every other test builds its own corpus, which is what keeps them stable.
        That is also what makes them blind to drift in the real artifact, so one
        test reads it.

        Two modules meet on this file and neither imports the other.
        `rules_condensed.core_members` decides which rules the core carries and
        writes their sections; `rule_coverage.always_on_rules` recovers those
        names by matching its own pattern against the written text. Let the
        writer change how it emits a rule heading and the reader silently
        recovers fewer names -- and rule-coverage then reports always-on rules as
        never-read, in every report, forever, which is the 87%-noise state
        `always_on_rules` exists to remove.

        This used to be checked through a `Rules: N of M` count in the file's own
        header. That number was derived data stored in a generated document (its
        M moved whenever ANY rule was added), so it went on 2026-08-18. Comparing
        the two modules directly is what the count was standing in for, and it is
        strictly stronger: it compares the NAMES, so a swap that keeps the total
        fixed no longer passes.
        """
        live = Path(__file__).resolve().parents[2] / "ai" / "rules"
        if not (live / "CORE.md").is_file():
            self.skipTest("no live ai/rules/CORE.md in this checkout")

        # The same inputs `build_core` uses. The corpus is not optional here:
        # without it `core_members` cannot derive the "no past task would
        # surface it" members, and the comparison would fail on the tool rather
        # than on the drift it is looking for.
        root = live.parents[1]
        written = {
            r["name"]
            for r in rules_condensed.core_members(
                rules_condensed.load_rules(live),
                corpus=rules_condensed.load_task_corpus(root),
            )
        }
        self.assertTrue(written, "the live corpus yields no always-on rule at all")
        self.assertEqual(
            written,
            rule_coverage.always_on_rules(live),
            "rules_condensed writes CORE.md sections that rule_coverage cannot "
            "parse back; one of the two changed format without the other",
        )

    def test_always_on_rule_is_never_reported_missed(self):
        """CORE.md carries its text, so no session can Read it or clear it.

        Without this, the same names are named in every report forever and the
        reader learns to skip the whole thing.
        """
        write_rule(
            self.rules, "spec-no-code.md", "writing or editing a spec", "blocking"
        )
        self.write_core("spec-no-code.md")
        write_transcript(
            self.transcript, [("Edit", str(self.root / "plan" / "spec-thing.md"))]
        )

        result = self.analyse()

        self.assertEqual([], result["missed"])
        self.assertNotIn("spec-no-code.md", result["matched"])
        self.assertEqual(1, result["always-on-excluded"])
        self.assertEqual(["spec-no-code.md"], result["always-on-rules"])
        self.assertEqual(0, result["blocking-total"], "it leaves the measured set")
        self.assertEqual(0, self.run_detector(), "an unclearable miss must not exit 1")

    def test_always_on_exclusion_is_stated_not_silent(self):
        """A guard that mutes something says so (ai/rules/evidence.md)."""
        write_rule(
            self.rules, "spec-no-code.md", "writing or editing a spec", "blocking"
        )
        self.write_core("spec-no-code.md")
        write_transcript(
            self.transcript, [("Edit", str(self.root / "plan" / "spec-thing.md"))]
        )

        text = rule_coverage.format_text(self.analyse(), Path("tmp/x.ndjson"))

        self.assertIn("1 always-on rule(s)", text)
        self.assertIn("CORE.md", text)

    def test_exclusion_is_scoped_to_core_not_a_blanket_mute(self):
        """A routable rule sharing the trigger is still reported.

        This is the test that would fail if the exclusion were widened to the
        trigger, the severity, or the file kind instead of CORE membership.
        """
        write_rule(
            self.rules, "spec-no-code.md", "writing or editing a spec", "blocking"
        )
        write_rule(
            self.rules, "planning.md", "when creating or updating a spec", "blocking"
        )
        self.write_core("spec-no-code.md")
        write_transcript(
            self.transcript, [("Edit", str(self.root / "plan" / "spec-thing.md"))]
        )

        result = self.analyse()

        self.assertEqual(["planning.md"], result["missed"])
        self.assertEqual(1, result["always-on-excluded"])
        self.assertEqual(1, self.run_detector(), "the genuine miss still exits 1")

    def test_missing_core_excludes_nothing_and_says_so(self):
        """No CORE.md must not silently mute; it must fall back to reporting.

        Both halves are asserted. The announcement is the guard's other half
        (`ai/rules/evidence.md`), and asserting only the behaviour
        leaves deleting the print() green.
        """
        write_rule(
            self.rules, "spec-no-code.md", "writing or editing a spec", "blocking"
        )
        write_transcript(
            self.transcript, [("Edit", str(self.root / "plan" / "spec-thing.md"))]
        )

        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            result = self.analyse()

        self.assertEqual(["spec-no-code.md"], result["missed"])
        self.assertEqual(0, result["always-on-excluded"])
        self.assertIn("cannot read", err.getvalue())
        self.assertIn("excluding no always-on rule", err.getvalue())

    def test_unparsable_core_excludes_nothing_and_says_so(self):
        """A CORE.md that exists but parses to nothing is the drift case.

        Reverting to over-reporting is safe; doing it quietly is not, because
        the operator cannot tell that state from the noise the exclusion was
        built to remove.
        """
        write_rule(
            self.rules, "spec-no-code.md", "writing or editing a spec", "blocking"
        )
        (self.rules / "CORE.md").write_text(
            "# Ze Rules -- Always-On Core\n\n## spec-no-code\n"
            "- [spec-no-code](ai/rules/spec-no-code.md)\n",
            encoding="utf-8",
        )
        write_transcript(
            self.transcript, [("Edit", str(self.root / "plan" / "spec-thing.md"))]
        )

        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            result = self.analyse()

        self.assertEqual(["spec-no-code.md"], result["missed"], "safe direction")
        self.assertEqual(0, result["always-on-excluded"])
        self.assertIn("readable but carries no", err.getvalue())
        self.assertIn("rules_condensed", err.getvalue(), "name the generator to check")

    def test_unmatchable_action_trigger_is_counted_not_hidden(self):
        """The blind spot is published, so silence is never read as coverage."""
        write_rule(
            self.rules,
            "never-destroy-work.md",
            "before deleting, reverting, or overwriting any file holding uncommitted work",
            "blocking",
        )
        write_transcript(
            self.transcript, [("Edit", str(self.root / "internal" / "wire.go"))]
        )

        result = self.analyse()

        self.assertEqual(
            [], result["missed"], "an action trigger cannot match a file type"
        )
        self.assertEqual(1, result["unmatchable"])
        self.assertIn("never-destroy-work.md", result["unmatchable-rules"])
        text = rule_coverage.format_text(result, Path("tmp/x.ndjson"))
        self.assertIn("UNDER-reports", text)

    def test_report_line_is_appended(self):
        """Evidence accumulates across sessions rather than scrolling away."""
        write_rule(
            self.rules, "performance.md", "writing any wire-encoding path", "blocking"
        )
        write_transcript(
            self.transcript, [("Edit", str(self.root / "internal" / "wire.go"))]
        )

        rc = rule_coverage.main(
            [
                "--transcript",
                str(self.transcript),
                "--rules-dir",
                str(self.rules),
                "--session",
                "sess-1",
            ]
        )

        self.assertEqual(1, rc)
        report = self.root / rule_coverage.REPORT_PATH
        rows = [json.loads(x) for x in report.read_text(encoding="utf-8").splitlines()]
        self.assertEqual(1, len(rows))
        self.assertEqual("sess-1", rows[0]["session"])
        self.assertEqual(["performance.md"], rows[0]["missed"])
        self.assertIn("unmatchable", rows[0])

    def test_unreadable_transcript_reports_nothing_and_exits_zero(self):
        """No observation means no verdict, never an invented one."""
        write_rule(
            self.rules, "performance.md", "writing any wire-encoding path", "blocking"
        )
        rc = rule_coverage.main(
            [
                "--transcript",
                str(self.root / "absent.jsonl"),
                "--rules-dir",
                str(self.rules),
                "--no-append",
            ]
        )
        self.assertEqual(0, rc)

    def test_unreadable_transcript_says_so_instead_of_going_quiet(self):
        """A transcript that exists but cannot be opened must speak.

        VALIDATES: `read_transcript` on an unreadable file writes a line naming
        the path and the error, and still returns empty sets.
        PREVENTS: the bare `except OSError: return written, rules_read`. Two
        empty sets are exactly what a genuinely read-only session produces, so
        the silent branch was indistinguishable from a real observation of
        nothing (ai/rules/evidence.md: a guard that cannot evaluate
        must say so). It stays advisory -- this hook never blocks a stop.
        """
        path = self.root / "locked.jsonl"
        path.write_text('{"tool_use": 1}\n', encoding="utf-8")
        os.chmod(path, 0o000)
        # tearDown removes the sandbox before addCleanup would fire, so the
        # permission is restored here rather than registered for later.
        try:
            try:
                with open(path, encoding="utf-8"):
                    self.skipTest("this user can read a 0o000 file (running as root)")
            except OSError:
                pass

            err = io.StringIO()
            with contextlib.redirect_stderr(err):
                written, rules_read = rule_coverage.read_transcript(str(path))
            self.assertEqual((set(), set()), (written, rules_read))
            self.assertIn("cannot read the session transcript", err.getvalue())
            self.assertIn(str(path), err.getvalue())
        finally:
            os.chmod(path, 0o644)


class TestNeverBlocksStop(DetectorCase):
    def _run_hook(self, payload):
        return subprocess.run(
            ["bash", str(HOOK)],
            input=json.dumps(payload),
            capture_output=True,
            text=True,
            check=False,
            cwd=str(self.root),
            env={**os.environ, "CLAUDE_PROJECT_DIR": str(self.root)},
        )

    def test_detector_never_blocks_stop(self):
        """Exit is 1 or 0 in every path. Exit 2 would refuse the session an end.

        Four shapes are exercised: a real miss, a clean session, an unparseable
        payload, and an empty one. None may return 2.
        """
        write_rule(
            self.rules, "performance.md", "writing any wire-encoding path", "blocking"
        )
        write_transcript(
            self.transcript, [("Edit", str(self.root / "internal" / "wire.go"))]
        )
        # The hook runs the checked-in detector, so give the sandbox a copy of it.
        (self.root / "scripts" / "dev").mkdir(parents=True, exist_ok=True)
        for mod in ("rule_coverage.py", "rules_lint.py", "running_model.py"):
            src = Path(__file__).resolve().parent / mod
            (self.root / "scripts" / "dev" / mod).write_text(
                src.read_text(encoding="utf-8"), encoding="utf-8"
            )

        cases = {
            "miss": {"transcript_path": str(self.transcript), "session_id": "s1"},
            "clean": {
                "transcript_path": str(self.root / "absent.jsonl"),
                "session_id": "s2",
            },
            "empty": {},
        }
        for name, payload in cases.items():
            with self.subTest(payload=name):
                proc = self._run_hook(payload)
                self.assertIn(
                    proc.returncode,
                    (0, 1),
                    f"{name}: got {proc.returncode}\n{proc.stderr}",
                )

        with self.subTest(payload="garbage"):
            proc = subprocess.run(
                ["bash", str(HOOK)],
                input="not json at all",
                capture_output=True,
                text=True,
                check=False,
                cwd=str(self.root),
                env={**os.environ, "CLAUDE_PROJECT_DIR": str(self.root)},
            )
            self.assertIn(proc.returncode, (0, 1), f"garbage: got {proc.returncode}")

    def test_hook_registered_after_the_blocking_stop_gate(self):
        """Placement is load-bearing: it must never mask block-premature-stop."""
        repo = Path(__file__).resolve().parents[2]
        settings = json.loads(
            (repo / ".claude" / "settings.json").read_text(encoding="utf-8")
        )
        commands = [
            h["command"] for group in settings["hooks"]["Stop"] for h in group["hooks"]
        ]
        blocker = [i for i, c in enumerate(commands) if "block-premature-stop.sh" in c]
        mine = [i for i, c in enumerate(commands) if "rule-coverage-report.sh" in c]
        self.assertTrue(blocker, "block-premature-stop.sh must stay registered on Stop")
        self.assertTrue(mine, "rule-coverage-report.sh must be registered on Stop")
        self.assertGreater(mine[0], blocker[0])


if __name__ == "__main__":
    unittest.main()
