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
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import rule_coverage

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
            self.rules, "buffer-first.md", "writing any wire-encoding path", "blocking"
        )
        write_transcript(
            self.transcript,
            [("Edit", str(self.root / "internal" / "component" / "bgp" / "wire.go"))],
        )

        result = self.analyse()

        self.assertIn("buffer-first.md", result["missed"])
        self.assertEqual(1, self.run_detector(), "a miss must exit 1, not 0")

    def test_detector_silent_when_all_read(self):
        """AC-11: reading the matched rule clears it."""
        write_rule(
            self.rules, "buffer-first.md", "writing any wire-encoding path", "blocking"
        )
        write_transcript(
            self.transcript,
            [
                ("Edit", str(self.root / "internal" / "wire.go")),
                ("Read", str(self.root / "ai" / "rules" / "buffer-first.md")),
            ],
        )

        result = self.analyse()

        self.assertIn("buffer-first.md", result["matched"])
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

        This is what makes a miss meaningful: CONDENSED.md is in every session's
        context today, so counting it would make the detector measure nothing.
        """
        write_rule(
            self.rules, "buffer-first.md", "writing any wire-encoding path", "blocking"
        )
        (self.rules / "CONDENSED.md").write_text("# digest\n", encoding="utf-8")
        write_transcript(
            self.transcript,
            [
                ("Edit", str(self.root / "internal" / "wire.go")),
                ("Read", str(self.rules / "CONDENSED.md")),
            ],
        )

        result = self.analyse()

        self.assertEqual(["buffer-first.md"], result["missed"])

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
            self.rules, "buffer-first.md", "writing any wire-encoding path", "blocking"
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
        self.assertEqual(["buffer-first.md"], rows[0]["missed"])
        self.assertIn("unmatchable", rows[0])

    def test_unreadable_transcript_reports_nothing_and_exits_zero(self):
        """No observation means no verdict, never an invented one."""
        write_rule(
            self.rules, "buffer-first.md", "writing any wire-encoding path", "blocking"
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
        nothing (ai/rules/fail-closed-guards.md: a guard that cannot evaluate
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
            self.rules, "buffer-first.md", "writing any wire-encoding path", "blocking"
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
