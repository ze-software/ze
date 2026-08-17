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
            "--rounds",
            "1",
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
            "--rounds",
            "1",
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
            "--rounds",
            "1",
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
            "--rounds",
            "1",
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
            "--rounds",
            "1",
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
            "--rounds",
            "1",
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
        # A human/off-session invocation with an unset or unsafe
        # CLAUDE_CODE_SESSION_ID (including a trailing newline, which a plain
        # `$`-anchored match would admit) must fall back to "shared" rather than
        # leak into the artifact filename. Clear the environment so a test run
        # from a live fork does not exercise the fork-only resolver path.
        sys.path.insert(0, str(HERE))
        import review_gate as rg
        from unittest import mock

        for bad in ("", "a b", "a/b", "abc\n"):
            with mock.patch.dict(
                os.environ, {"CLAUDE_CODE_SESSION_ID": bad}, clear=True
            ):
                self.assertEqual(rg.session_id(), "shared", f"{bad!r} should fall back")
        with mock.patch.dict(
            os.environ, {"CLAUDE_CODE_SESSION_ID": "uuid-12ab"}, clear=True
        ):
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


class SpecStemAcceptsEverySpelling(unittest.TestCase):
    """VALIDATES: --spec resolves the same artifact from all three spellings.

    PREVENTS: the silent false BLOCKED. artifact_path() stripped only the
    "spec-" prefix, so a leading "plan/" survived and
    `--spec plan/spec-X.md` resolved to tmp/review/plan/spec-X-<session>.md.
    That directory does not exist, so the tool reported BLOCKED and advised
    running an independent review -- the one remedy that could not help, since
    the review HAD been run and its artifact was sitting in tmp/review/ under
    the correct name. commit_helper.py was immune because it derives the stem
    itself, so only direct callers saw it, and they read it as a missing review.

    The path form is the one ze-close and commit_helper.py both carry, so it is
    the spelling a caller is most likely to paste.
    """

    def test_all_three_spellings_agree(self):
        sys.path.insert(0, str(Path(__file__).resolve().parent))
        import review_gate as rg

        want = "gokrazy-init-bump"
        for spelling in (
            "plan/spec-gokrazy-init-bump.md",
            "spec-gokrazy-init-bump.md",
            "gokrazy-init-bump",
        ):
            with self.subTest(spelling=spelling):
                self.assertEqual(rg.spec_stem(spelling), want)

    def test_artifact_path_carries_no_directory(self):
        sys.path.insert(0, str(Path(__file__).resolve().parent))
        import review_gate as rg

        # The bug was invisible in the stem and visible only in the path: a
        # surviving "plan/" turned one filename into a subdirectory nobody
        # creates, so assert on the parent rather than on the name alone.
        p = rg.artifact_path("plan/spec-gokrazy-init-bump.md")
        self.assertEqual(p.parent, rg.ARTIFACT_DIR)
        self.assertTrue(p.name.startswith("gokrazy-init-bump-"), p.name)


class RoundCapCase(unittest.TestCase):
    """The review loop's round cap.

    VALIDATES: `record` demands the round count, writes it into the artifact, and
    refuses more than ROUND_CAP rounds unless the operator names the PRODUCT defect
    a later round found.
    PREVENTS: the failure this cap was added for. On 2026-08-09 a test-only change
    took seven review passes; the code was clean after pass 1 and all eleven later
    findings were false statements in the spec's own closure prose. Each round's
    fixes were prose, so each next round had fresh prose to audit and the loop had
    no state in which it stopped.
    """

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        (self.root / "tmp" / "review").mkdir(parents=True)
        self.code = self.root / "pkg" / "a.go"
        self.code.parent.mkdir()
        self.code.write_text("package a\nfunc A() {}\n")

    def tearDown(self):
        self.tmp.cleanup()

    def record(self, *extra):
        return run_gate(
            "record",
            "--spec",
            "demo",
            "--verdict",
            "clean",
            "--files",
            "pkg/a.go",
            *extra,
            cwd=self.root,
        )

    def artifact(self):
        (only,) = (self.root / "tmp" / "review").glob("demo-*.md")
        return only.read_text()

    def test_rounds_is_required(self):
        r = self.record()
        self.assertEqual(r.returncode, 2)
        self.assertIn("--rounds", r.stderr)

    def test_rounds_under_the_cap_records_and_is_written_down(self):
        r = self.record("--rounds", "3")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("rounds=3", self.artifact())

    def test_over_the_cap_is_refused_without_a_reason(self):
        r = self.record("--rounds", "4")
        self.assertEqual(r.returncode, 2)
        self.assertIn("--rounds-reason", r.stderr)
        # The refusal must say what a valid reason IS, or the next agent writes
        # "the review found more issues", which is the thing being refused.
        self.assertIn("product", r.stderr.lower())

    def test_over_the_cap_records_when_a_product_defect_is_named(self):
        r = self.record(
            "--rounds",
            "5",
            "--rounds-reason",
            "round 4 found the retry loop drops the last error",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        art = self.artifact()
        self.assertIn("rounds=5", art)
        self.assertIn("round 4 found the retry loop drops the last error", art)

    def test_a_blank_reason_does_not_lift_the_cap(self):
        r = self.record("--rounds", "4", "--rounds-reason", "   ")
        self.assertEqual(r.returncode, 2)

    def test_zero_rounds_is_refused(self):
        # An artifact claiming zero passes is a review that never ran.
        r = self.record("--rounds", "0")
        self.assertEqual(r.returncode, 2)

    def test_check_accepts_an_artifact_that_predates_the_cap(self):
        # Artifacts recorded before --rounds existed carry no rounds= field, and
        # `check` must not start failing on them: the cap governs RECORDING.
        self.record("--rounds", "1")
        (only,) = (self.root / "tmp" / "review").glob("demo-*.md")
        only.write_text(only.read_text().replace(" rounds=1", ""))
        r = run_gate("check", "--spec", "demo", "--files", "pkg/a.go", cwd=self.root)
        self.assertEqual(r.returncode, 0, r.stderr)


if __name__ == "__main__":
    unittest.main()
