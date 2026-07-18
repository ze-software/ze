#!/usr/bin/env python3
"""Tests for the independent-review gate (scripts/dev/review_gate.py) and the
commit_helper spec-closure integration. Run: python3 scripts/dev/review_gate_test.py

VALIDATES: a spec-closure commit is BLOCKED unless a CLEAN artifact covers the
committed code files and their hashes still match; a post-review edit re-opens the
gate; a not-clean verdict blocks; a code file the review never saw blocks.
PREVENTS: the recurring failure where an author narrates "0 issues" into the spec
and closes it without an independent review (ai/rationale/critical-review.md).
"""

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
GATE = HERE / "review_gate.py"
HELPER = HERE / "commit_helper.py"


def run_gate(*args, cwd):
    return subprocess.run(
        [sys.executable, str(GATE), *args], cwd=cwd, capture_output=True, text=True
    )


class ReviewGateCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        # review_gate.py resolves paths relative to CWD, so run inside self.root.
        (self.root / "tmp" / "review").mkdir(parents=True)
        self.code = self.root / "pkg" / "a.go"
        self.code.parent.mkdir()
        self.code.write_text("package a\nfunc A() {}\n")

    def tearDown(self):
        self.tmp.cleanup()

    def rel(self, p: Path) -> str:
        return str(p.relative_to(self.root))

    def test_missing_artifact_blocks(self):
        r = run_gate(
            "check", "--spec", "demo", "--files", self.rel(self.code), cwd=self.root
        )
        self.assertEqual(r.returncode, 3)
        self.assertIn("no independent-review artifact", r.stderr)

    def test_record_then_check_passes(self):
        run_gate(
            "record",
            "--spec",
            "demo",
            "--verdict",
            "clean",
            "--files",
            self.rel(self.code),
            cwd=self.root,
        )
        r = run_gate(
            "check", "--spec", "demo", "--files", self.rel(self.code), cwd=self.root
        )
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_edit_after_review_blocks_stale(self):
        run_gate(
            "record",
            "--spec",
            "demo",
            "--verdict",
            "clean",
            "--files",
            self.rel(self.code),
            cwd=self.root,
        )
        self.code.write_text("package a\nfunc A() {}\nfunc B() {}\n")
        r = run_gate(
            "check", "--spec", "demo", "--files", self.rel(self.code), cwd=self.root
        )
        self.assertEqual(r.returncode, 3)
        self.assertIn("changed AFTER the review", r.stderr)

    def test_findings_verdict_blocks(self):
        run_gate(
            "record",
            "--spec",
            "demo",
            "--verdict",
            "findings",
            "--files",
            self.rel(self.code),
            cwd=self.root,
        )
        r = run_gate(
            "check", "--spec", "demo", "--files", self.rel(self.code), cwd=self.root
        )
        self.assertEqual(r.returncode, 3)
        self.assertIn("not clean", r.stderr)

    def test_unreviewed_code_file_blocks(self):
        other = self.root / "pkg" / "b.go"
        other.write_text("package a\n")
        run_gate(
            "record",
            "--spec",
            "demo",
            "--verdict",
            "clean",
            "--files",
            self.rel(self.code),
            cwd=self.root,
        )
        r = run_gate(
            "check",
            "--spec",
            "demo",
            "--files",
            self.rel(self.code),
            self.rel(other),
            cwd=self.root,
        )
        self.assertEqual(r.returncode, 3)
        self.assertIn("not covered by the review", r.stderr)

    def test_noncode_files_do_not_require_review(self):
        # A closure carrying only docs/generated files has nothing to critically
        # review, so `check` over only a .md file passes even with no artifact.
        doc = self.root / "readme.md"
        doc.write_text("# x\n")
        run_gate(
            "record",
            "--spec",
            "demo",
            "--verdict",
            "clean",
            "--files",
            self.rel(self.code),
            cwd=self.root,
        )
        r = run_gate("check", "--spec", "demo", "--files", self.rel(doc), cwd=self.root)
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_different_sessions_do_not_share_artifact(self):
        # F2: the artifact is session-scoped. Session A records clean and A's own
        # check passes, but a concurrent session B (different CLAUDE_CODE_SESSION_ID)
        # on the SAME spec has no artifact of its own and is BLOCKED -- so two
        # agents on one spec can neither clobber nor ride each other's review.
        def gate(sid, *args):
            env = {**os.environ, "CLAUDE_CODE_SESSION_ID": sid}
            return subprocess.run(
                [sys.executable, str(GATE), *args],
                cwd=self.root,
                env=env,
                capture_output=True,
                text=True,
            )

        gate(
            "sessionA",
            "record",
            "--spec",
            "demo",
            "--verdict",
            "clean",
            "--files",
            self.rel(self.code),
        )
        ra = gate("sessionA", "check", "--spec", "demo", "--files", self.rel(self.code))
        self.assertEqual(ra.returncode, 0, ra.stderr)
        rb = gate("sessionB", "check", "--spec", "demo", "--files", self.rel(self.code))
        self.assertEqual(rb.returncode, 3)
        self.assertIn("no independent-review artifact", rb.stderr)

    def test_session_id_falls_back_on_unsafe_env(self):
        # An unset or unsafe CLAUDE_CODE_SESSION_ID (including a trailing newline,
        # which a plain `$`-anchored match would admit) must fall back to "shared"
        # rather than leak into the artifact filename.
        sys.path.insert(0, str(HERE))
        import review_gate as rg
        from unittest import mock

        for bad in ("", "a b", "a/b", "abc\n"):
            with mock.patch.dict(os.environ, {"CLAUDE_CODE_SESSION_ID": bad}):
                self.assertEqual(rg.session_id(), "shared", f"{bad!r} should fall back")
        with mock.patch.dict(os.environ, {"CLAUDE_CODE_SESSION_ID": "uuid-12ab"}):
            self.assertEqual(rg.session_id(), "uuid-12ab")


class CommitHelperIntegrationCase(unittest.TestCase):
    """spec_closure_stem drives which commits the review gate applies to."""

    def test_stem_from_learned_and_spec(self):
        sys.path.insert(0, str(HERE))
        import commit_helper as ch

        self.assertEqual(
            ch.spec_closure_stem(
                ("plan/learned/1169-cli-root-namespace-grammar.md",), ()
            ),
            "cli-root-namespace-grammar",
        )
        self.assertEqual(
            ch.spec_closure_stem((), ("plan/spec-cli-root-namespace-grammar.md",)),
            "cli-root-namespace-grammar",
        )
        # A non-closure commit (no learned add, no spec remove) is unaffected.
        self.assertIsNone(ch.spec_closure_stem(("internal/x.go",), ()))


if __name__ == "__main__":
    unittest.main()
