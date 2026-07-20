#!/usr/bin/env python3
"""Unit tests for the discovery-index commit gate in commit_helper.py."""

from __future__ import annotations

import os
import re
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
import commit_helper as ch


def _git(root: Path, *args: str) -> None:
    subprocess.run(["git", *args], cwd=root, check=True, capture_output=True, text=True)


class TestFeedsDiscoveryIndex(unittest.TestCase):
    def test_paths(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "internal" / "a").mkdir(parents=True)
            (root / "internal" / "a" / "a.go").write_text(
                "// Package a does A.\npackage a\n"
            )
            (root / "internal" / "b").mkdir(parents=True)
            (root / "internal" / "b" / "b.go").write_text("package b\n\nfunc X() {}\n")

            f = ch.feeds_discovery_index
            self.assertTrue(f(root, "internal/x/register.go"))
            self.assertTrue(f(root, "plan/learned/1099-z.md"))
            self.assertTrue(f(root, "scripts/dev/package_map.py"))
            self.assertTrue(f(root, "ai/PACKAGE-MAP.md"))
            self.assertTrue(f(root, "Makefile"))
            self.assertTrue(f(root, "mk/inventory.mk"))
            self.assertTrue(f(root, "internal/a/a.go"))  # header present
            self.assertFalse(f(root, "internal/b/b.go"))  # no // Package/// Design:
            self.assertFalse(f(root, "internal/a/a_test.go"))
            self.assertFalse(f(root, "docs/guide/x.md"))


class TestIndexPending(unittest.TestCase):
    def test_new_committed_modified(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init", "-q")
            _git(root, "config", "user.email", "t@example.com")
            _git(root, "config", "user.name", "t")
            _git(root, "config", "commit.gpgsign", "false")
            (root / "ai").mkdir()
            rel = "ai/PACKAGE-MAP.md"
            (root / rel).write_text("v1\n")

            self.assertTrue(ch.index_pending(root, rel))  # untracked -> pending
            _git(root, "add", rel)
            _git(root, "commit", "-qm", "init")
            self.assertFalse(ch.index_pending(root, rel))  # clean -> not pending
            (root / rel).write_text("v2\n")
            self.assertTrue(ch.index_pending(root, rel))  # modified -> pending


FAKEGEN = textwrap.dedent("""\
    import sys
    from pathlib import Path
    root = Path(__file__).resolve().parents[2]
    out = root / "ai" / "FAKE.md"
    src = root / "src"
    names = sorted(p.name for p in src.glob("*.txt")) if src.is_dir() else []
    content = ",".join(names)
    if "--check" in sys.argv:
        cur = out.read_text() if out.exists() else ""
        if cur != content:
            print("is stale")
            sys.exit(1)
        sys.exit(0)
    out.write_text(content)
""")


class TestHeadStatus(unittest.TestCase):
    def test_detects_committed_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init", "-q")
            _git(root, "config", "user.email", "t@example.com")
            _git(root, "config", "user.name", "t")
            _git(root, "config", "commit.gpgsign", "false")
            (root / "scripts" / "dev").mkdir(parents=True)
            (root / "src").mkdir()
            (root / "ai").mkdir()
            (root / "scripts" / "dev" / "fakegen.py").write_text(FAKEGEN)
            (root / "src" / "a.txt").write_text("a\n")
            (root / "ai" / "FAKE.md").write_text("a.txt")  # consistent with HEAD src
            _git(root, "add", "-A")
            _git(root, "commit", "-qm", "init")

            saved = (ch.DISCOVERY_INDEX_GENERATORS, ch.DISCOVERY_INDEX_OUTPUTS)
            ch.DISCOVERY_INDEX_GENERATORS = ("scripts/dev/fakegen.py",)
            ch.DISCOVERY_INDEX_OUTPUTS = ("ai/FAKE.md",)
            try:
                self.assertEqual(ch.discovery_index_head_status(root)[0], "fresh")
                # Bypass: commit a new source without updating the committed index.
                (root / "src" / "b.txt").write_text("b\n")
                _git(root, "add", "src/b.txt")
                _git(root, "commit", "-qm", "bypass")
                self.assertEqual(
                    ch.discovery_index_head_status(root), ("stale", ["ai/FAKE.md"])
                )
            finally:
                ch.DISCOVERY_INDEX_GENERATORS, ch.DISCOVERY_INDEX_OUTPUTS = saved

    def test_unknown_without_head(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init", "-q")
            self.assertEqual(ch.discovery_index_head_status(root)[0], "unknown")


DEFERRALS_HEADER = (
    "| Date | Source | What | Reason | Destination | Status |\n"
    "|------|--------|------|--------|-------------|--------|\n"
)


class TestDeferralDestination(unittest.TestCase):
    """ai/rules/deferral-tracking.md: an open deferral always names a spec that exists."""

    def _repo(self, tmp: str, rows: str) -> Path:
        root = Path(tmp)
        (root / "plan").mkdir(parents=True)
        (root / "plan" / "spec-rib-deferred-ipv6-coverage.md").write_text("# Spec\n")
        (root / "plan" / "deferrals.md").write_text(DEFERRALS_HEADER + rows)
        return root

    def _row(self, dest: str) -> str:
        return (
            f"| 2026-07-16 | spec-rib.md | IPv6 flush path | time | {dest} | open |\n"
        )

    def test_existing_spec_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(
                tmp, self._row("`plan/spec-rib-deferred-ipv6-coverage.md`")
            )
            self.assertEqual(ch.deferral_unassigned_problems(root), [])

    def test_cancelled_needs_no_spec(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp, self._row("user-approved-drop"))
            self.assertEqual(ch.deferral_unassigned_problems(root), [])

    def test_placeholder_flagged(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp, self._row("-"))
            problems = ch.deferral_unassigned_problems(root)
            self.assertEqual(len(problems), 1)
            self.assertIn("no destination", problems[0])

    def test_prose_destination_flagged(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp, self._row("future work, once the RIB settles"))
            problems = ch.deferral_unassigned_problems(root)
            self.assertEqual(len(problems), 1)
            self.assertIn("names no file", problems[0])

    def test_missing_spec_file_flagged(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(
                tmp, self._row("`plan/spec-rib-deferred-nobody-wrote-it.md`")
            )
            problems = ch.deferral_unassigned_problems(root)
            self.assertEqual(len(problems), 1)
            self.assertIn("does not exist", problems[0])

    # VALIDATES: a real destination is not flagged because its prose also names a
    # spec that was deleted at closure. ONE named file existing is a home.
    # PREVENTS: the false positive this check shipped with. Two correctly-homed
    # rows in plan/deferrals.md cite their retired original destination to
    # explain a re-homing; requiring every filename to resolve flagged both, and
    # the cheapest way to silence it would have been to delete the provenance.
    def test_existing_destination_with_retired_spec_in_prose_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(
                tmp,
                self._row(
                    "`plan/spec-rib-deferred-ipv6-coverage.md` (re-homed 2026-07-16: "
                    "the original destination `spec-lg-birdwatcher-peer-fields.md` "
                    "was deleted at closure, orphaning this row)"
                ),
            )
            self.assertEqual(ch.deferral_unassigned_problems(root), [])

    # VALIDATES: a nested plan/ path (a learned summary) is read as one path.
    # PREVENTS: a regex that only spans [\w.-] silently matching nothing on
    # plan/learned/1127-x.md and calling a real destination "no file".
    def test_nested_plan_path_destination_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp, self._row("`plan/learned/1127-rib-arch-2.md`"))
            (root / "plan" / "learned").mkdir(parents=True, exist_ok=True)
            (root / "plan" / "learned" / "1127-rib-arch-2.md").write_text("# L\n")
            self.assertEqual(ch.deferral_unassigned_problems(root), [])

    def test_resolved_rows_are_not_checked(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir(parents=True)
            (root / "plan" / "deferrals.md").write_text(
                DEFERRALS_HEADER
                + "| 2026-07-16 | spec-rib.md | old item | time | later maybe | done |\n"
            )
            self.assertEqual(ch.deferral_unassigned_problems(root), [])


def _deferral_gate(rows: list[tuple[str, str]]) -> list[str]:
    """Run deferral_unassigned_problems over a table of (destination, status).

    Any plan/*.md named by a destination is created, so these cases isolate the
    STATUS half: a row that fails here fails because of its status, never because
    its spec happens not to exist in the fixture. Resolution goes through
    ch.deferral_destination_paths so the harness creates exactly the paths the
    gate checks; resolving it a second time here would let the two drift.
    """
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        (root / "plan").mkdir()
        body = ""
        for i, (dest, status) in enumerate(rows):
            for ref in ch.deferral_destination_paths(dest):
                (root / ref).parent.mkdir(parents=True, exist_ok=True)
                (root / ref).write_text("# Spec\n")
            body += f"| 2026-07-16 | spec-x | what-{i} | reason | {dest} | {status} |\n"
        (root / "plan" / "deferrals.md").write_text(DEFERRALS_HEADER + body)
        return ch.deferral_unassigned_problems(root)


class TestDeferralUnassigned(unittest.TestCase):
    """The STATUS half of the gate enforcing ai/rules/deferral-tracking.md's "no
    deferral without a destination".

    TestDeferralDestination above covers which Destination cells are a valid home.
    This class covers WHICH ROWS ARE LOOKED AT AT ALL, which is the half that was
    fail-open (ai/rules/fail-closed-guards.md): the gate tested `status == "open"`,
    so every row at `deferred` bypassed the destination check no matter how strict
    that check became.
    """

    # VALIDATES: a row at status `deferred` with no destination is flagged.
    # `deferred` is the word ai/rules/deferral-tracking.md itself uses for this
    # state, and 40 of the 68 rows in plan/deferrals.md carry it.
    # PREVENTS: hole 1 -- `status == "open"` meant a row written in the rule's own
    # vocabulary was never looked at.
    def test_status_hole_deferred_with_placeholder_destination(self):
        self.assertTrue(_deferral_gate([("none", "deferred")]))

    # VALIDATES: `deferred` + a prose destination is flagged. This is the exact
    # shape of the four rows written on 2026-07-16: they named no home, in the
    # vocabulary the rule teaches, and the gate never looked.
    def test_status_hole_deferred_with_prose_destination(self):
        self.assertTrue(_deferral_gate([("none yet (future spec)", "deferred")]))

    # VALIDATES: an unrecognised status is treated as live and checked.
    # PREVENTS: the fix re-running the original bug. An allowlist of live statuses
    # would skip the next word someone invents, which is precisely how `deferred`
    # got through; the terminal denylist fails closed instead.
    def test_unknown_status_is_checked_not_skipped(self):
        self.assertTrue(_deferral_gate([("none yet", "parked")]))

    # VALIDATES: a terminal status is exempt even with a placeholder destination.
    # plan/deferrals.md really carries `none (permanent exclusion; ...)` at
    # `cancelled`; flagging it would make the gate noise.
    # PREVENTS: over-reach -- a gate that flags everything is as useless as one
    # that flags nothing. This is why the status half must be bounded, not removed.
    def test_must_not_fire_terminal_status_with_placeholder(self):
        self.assertEqual(
            _deferral_gate(
                [
                    (
                        "none (permanent exclusion; AC-5 covers in-tree pages)",
                        "cancelled",
                    ),
                    ("none yet", "done"),
                    ("-", "resolved"),
                ]
            ),
            [],
        )

    # VALIDATES: a real, existing destination spec at a live status is not flagged.
    # Widening the status half must not turn the 28 correctly-homed live rows red.
    def test_must_not_fire_real_destination_at_live_status(self):
        self.assertEqual(
            _deferral_gate(
                [
                    ("`plan/spec-finish-l2tp.md` (work item added)", "deferred"),
                    ("`plan/spec-fixit-x.md` (F4)", "open"),
                    ("`plan/learned/1127-rib-arch-2.md`", "in-progress (spec)"),
                ]
            ),
            [],
        )

    # VALIDATES: the original `open` behavior still holds (regression guard).
    def test_open_with_empty_destination_still_flagged(self):
        self.assertTrue(_deferral_gate([("", "open")]))

    # VALIDATES: a row that cannot be parsed is reported rather than skipped.
    # PREVENTS: hole 3 -- `len(fields) < 7: continue` dropped a short row on the
    # floor and a Status-less row read status as "" and passed, so a malformed row
    # was silently treated as absent-and-fine (ai/rules/fail-closed-guards.md:
    # a guard that neither denies nor speaks does not exist).
    def test_malformed_row_is_reported_not_skipped(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "deferrals.md").write_text(
                DEFERRALS_HEADER
                + "| 2026-07-16 | spec-x | dropped work | reason | dest-only |\n"
            )
            problems = ch.deferral_unassigned_problems(root)
            self.assertTrue(problems, "a Status-less row must not pass silently")
            self.assertIn("malformed", "\n".join(problems).lower())

    # VALIDATES: the table's own header and separator are not parsed as rows.
    # PREVENTS: the separator's `-------------` destination being read as the `-`
    # placeholder once the destination match widens.
    def test_header_and_separator_are_not_rows(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "deferrals.md").write_text(DEFERRALS_HEADER)
            self.assertEqual(ch.deferral_unassigned_problems(root), [])


class TestDeferralGateSeverity(unittest.TestCase):
    """An unhomed deferral is ADVISORY: its message rides commit_gate_warnings,
    never commit_gate_problems, so a bookkeeping-only issue (harmless to software
    behaviour) cannot hard-block a commit (ai/rules/deferral-tracking.md). The
    detector still flags it, so nothing goes silent.
    """

    def _repo_with_unhomed_row(self, tmp: str) -> Path:
        root = Path(tmp)
        (root / "plan").mkdir(parents=True)
        (root / "plan" / "deferrals.md").write_text(
            DEFERRALS_HEADER
            + "| 2026-07-16 | spec-x | dropped work | reason | future work | deferred |\n"
        )
        return root

    def test_detector_still_flags_the_unhomed_row(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo_with_unhomed_row(tmp)
            self.assertTrue(ch.deferral_unassigned_problems(root))

    def test_unhomed_row_is_not_a_block_gate(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo_with_unhomed_row(tmp)
            problems = " ".join(ch.commit_gate_problems(root, (), ())).lower()
            self.assertNotIn(
                "live deferrals",
                problems,
                "an unhomed deferral must not appear among BLOCK-severity gates",
            )

    def test_unhomed_row_is_surfaced_as_a_warning(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo_with_unhomed_row(tmp)
            warnings = " ".join(ch.commit_gate_warnings(root, ())).lower()
            self.assertIn(
                "live deferrals",
                warnings,
                "an unhomed deferral must still be surfaced as a warning",
            )


class TestDeferralInDiff(unittest.TestCase):
    """deferral_in_diff_problems scans added prose for un-homed deferral language,
    but exempts the rule corpus (ai/rules/, .claude/rules/), which DISCUSSES
    deferral policy rather than parking work. Code and specs stay in scope
    (ai/rules/deferral-tracking.md, ai/rules/friction-reporting.md).
    """

    def _repo(self, tmp: str) -> Path:
        root = Path(tmp)
        _git(root, "init")
        _git(root, "config", "user.email", "t@e.st")
        _git(root, "config", "user.name", "t")
        (root / "README.md").write_text("seed\n")
        _git(root, "add", "README.md")
        _git(root, "commit", "-q", "-m", "seed")
        return root

    def test_bare_deferral_in_code_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "internal").mkdir()
            (root / "internal" / "x.go").write_text(
                "package x\n// out of scope for this change; future work\n"
            )
            self.assertTrue(ch.deferral_in_diff_problems(root, ("internal/x.go",), ()))

    def test_bare_deferral_in_rule_doc_is_exempt(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "ai" / "rules").mkdir(parents=True)
            (root / "ai" / "rules" / "foo.md").write_text(
                "# Foo\n\nThis rule governs future work and out of scope items.\n"
            )
            self.assertEqual(
                ch.deferral_in_diff_problems(root, ("ai/rules/foo.md",), ()), []
            )

    def test_bare_deferral_in_generated_condensed_is_exempt(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "ai" / "rules").mkdir(parents=True)
            (root / "ai" / "rules" / "CONDENSED.md").write_text(
                "# Condensed\n\nRisks: new coupling, follow-up work, future work.\n"
            )
            self.assertEqual(
                ch.deferral_in_diff_problems(root, ("ai/rules/CONDENSED.md",), ()), []
            )

    def test_bare_deferral_in_spec_still_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-x.md").write_text(
                "# Spec\n\nThe IPv6 path is future work.\n"
            )
            self.assertTrue(ch.deferral_in_diff_problems(root, ("plan/spec-x.md",), ()))


class TestStagingGuard(unittest.TestCase):
    """render_staging_guard aborts a generated commit script when the shared index
    holds files this commit did not stage (concurrent-session cross-commit guard)."""

    def _run_guard(self, root: Path, paths: tuple[str, ...]):
        script = (
            "#!/bin/bash\nset -euo pipefail\n" + ch.render_staging_guard(paths) + "\n"
        )
        return subprocess.run(
            ["bash", "-c", script], cwd=root, capture_output=True, text=True
        )

    def test_aborts_on_foreign_staged_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init")
            (root / "mine.txt").write_text("mine")
            (root / "foreign.txt").write_text("foreign")
            _git(
                root, "add", "mine.txt", "foreign.txt"
            )  # a sibling's file is also staged
            r = self._run_guard(root, ("mine.txt",))
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertIn("foreign.txt", r.stderr)

    def test_passes_when_only_own_files_staged(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init")
            (root / "mine.txt").write_text("mine")
            _git(root, "add", "mine.txt")
            r = self._run_guard(root, ("mine.txt",))
            self.assertEqual(r.returncode, 0, r.stderr)

    def test_no_false_abort_on_non_ascii_own_file(self):
        # git quotePath would C-quote a non-ASCII path; the guard disables it so a
        # legitimate commit of café.txt is not mis-flagged as a foreign staged file.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init")
            (root / "café.txt").write_text("x")
            _git(root, "add", "café.txt")
            r = self._run_guard(root, ("café.txt",))
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_no_false_abort_when_expected_file_unchanged(self):
        # An expected path with no staged diff must NOT trigger a false abort.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init")
            # Disable commit signing (matching every sibling commit test in this
            # file): a global commit.gpgsign=true would otherwise make this the
            # only test that fails when gpg cannot reach a tty for a passphrase.
            _git(root, "config", "commit.gpgsign", "false")
            (root / "a.txt").write_text("a")
            _git(root, "add", "a.txt")
            _git(
                root,
                "-c",
                "user.email=t@t",
                "-c",
                "user.name=t",
                "commit",
                "-m",
                "init",
            )
            r = self._run_guard(root, ("a.txt",))
            self.assertEqual(r.returncode, 0, r.stderr)


class TestDiscoveryIndexProblems(unittest.TestCase):
    """T-6: the freshness gate demands ONLY the indexes a commit's sources feed.

    An index left dirty by a concurrent session (one THIS commit does not feed)
    must not be demanded -- following that demand cross-commits another session's
    index row, the exact failure git-safety.md documents. A genuinely omitted
    index that this commit DOES feed must still be demanded (AC-9, both
    directions). discovery_index_freshness runs the real generators, so it is
    stubbed to isolate the fresh-on-disk branch this fix touches.
    """

    def _repo(self, tmp: str) -> Path:
        root = Path(tmp)
        _git(root, "init", "-q")
        _git(root, "config", "user.email", "t@example.com")
        _git(root, "config", "user.name", "t")
        _git(root, "config", "commit.gpgsign", "false")
        (root / "ai").mkdir()
        (root / "plan" / "learned").mkdir(parents=True)
        for out in ch.DISCOVERY_INDEX_OUTPUTS:
            (root / out).write_text("v1\n")
        _git(root, "add", "-A")
        _git(root, "commit", "-qm", "init")
        return root

    def test_unrelated_dirty_index_passes(self):
        saved = ch.discovery_index_freshness
        ch.discovery_index_freshness = lambda repo: ("fresh", [])
        try:
            with tempfile.TemporaryDirectory() as tmp:
                root = self._repo(tmp)
                # This commit feeds ONLY the learned index and includes it.
                (root / "plan" / "learned" / "1200-x.md").write_text("# L\n")
                (root / "ai" / "LEARNED-FULL-INDEX.md").write_text("v2\n")
                # DOCS-TO-CODE.md is dirty from another session but this commit
                # does not feed it, so the gate must not demand it.
                (root / "ai" / "DOCS-TO-CODE.md").write_text("someone-else\n")
                problems = ch.discovery_index_problems(
                    root,
                    ("plan/learned/1200-x.md", "ai/LEARNED-FULL-INDEX.md"),
                )
                self.assertEqual(problems, [], problems)
        finally:
            ch.discovery_index_freshness = saved

    def test_fed_index_omitted_still_refuses(self):
        saved = ch.discovery_index_freshness
        ch.discovery_index_freshness = lambda repo: ("fresh", [])
        try:
            with tempfile.TemporaryDirectory() as tmp:
                root = self._repo(tmp)
                # Commit feeds the learned index, whose output IS dirty but omitted.
                (root / "plan" / "learned" / "1200-x.md").write_text("# L\n")
                (root / "ai" / "LEARNED-FULL-INDEX.md").write_text("v2\n")
                problems = ch.discovery_index_problems(
                    root, ("plan/learned/1200-x.md",)
                )
                self.assertTrue(problems, "a fed-but-omitted dirty index must refuse")
                self.assertIn("LEARNED-FULL-INDEX.md", "\n".join(problems))
        finally:
            ch.discovery_index_freshness = saved


class TestStructuralGateRemediation(unittest.TestCase):
    """T-3 (AC-5): the structural-gate refusal must name a command that actually
    refreshes tmp/ze-verify-failures.json. Only a full `make ze-verify` /
    `ze-verify-changed` (verify_run.go) rewrites that record; `make <gate>` alone
    does not. A remediation that cannot work is worse than none (ai/rules/error-messages.md).
    """

    def test_refusal_names_the_real_refresher_and_disclaims_the_gate_command(self):
        import contextlib
        import io

        saved = (ch.verify_status, ch.structural_gate_reds)
        ch.verify_status = lambda repo: ("stale", "structural red")
        ch.structural_gate_reds = lambda repo: ["ze-lint-changed"]
        try:
            with tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                _git(root, "init", "-q")
                _git(root, "config", "user.email", "t@example.com")
                _git(root, "config", "user.name", "t")
                _git(root, "config", "commit.gpgsign", "false")
                (root / "f.txt").write_text("hello\n")
                err = io.StringIO()
                with contextlib.redirect_stderr(err):
                    rc = ch.main(
                        [
                            "--repo",
                            str(root),
                            "create",
                            "--session",
                            "abcd1234",
                            "--subject",
                            "fixture",
                            "--file",
                            "f.txt",
                            "--lesson-not-needed",
                            "fixture test for the structural-gate remediation text",
                        ]
                    )
                msg = err.getvalue()
                self.assertEqual(rc, 2, msg)
                # Names the TRUE refresher.
                self.assertIn("ze-verify", msg)
                # And makes explicit the per-gate command does NOT refresh the record.
                self.assertRegex(msg, r"ze-verify-failures\.json")
                self.assertIn("NOT", msg)
        finally:
            ch.verify_status, ch.structural_gate_reds = saved


class TestStructuralGatesAreLiveStages(unittest.TestCase):
    """STRUCTURAL_GATES must only name stages `make ze-verify` actually runs.

    structural_gate_reds() matches these names against the `stage` field of
    tmp/ze-verify-failures.json, which verify_run.go fills from stagesForMode().
    A name absent from stagesForMode can never match, so it silently gates
    nothing while reading as a live safety net -- exactly what
    `ze-cli-grammar-check` did (it is a real make target in mk/inventory.mk, but
    was only ever a stage of the dead _ze-verify-impl targets).

    The live names are PARSED out of stagesForMode rather than restated here:
    a second hand-kept copy of the stage list is the bug this test exists to
    prevent (ai/rules/derive-not-hardcode.md). That is the opposite choice from
    the goldens in verify_run_test.go, and deliberately so -- a golden's job is
    to be a change-detector for the list itself, while this test only needs to
    know what the list currently CONTAINS in order to check a subset relation.

    CACHING: this file runs as a python3 subprocess under TestPythonUnitTests
    (scripts/dev/python_tests_test.go), so verify_run.go is NOT a `go test`
    cache input for it. Editing verify_run.go alone can therefore serve a cached
    PASS here (and under ze-verify-changed, changed-pkgs.sh maps a *.go edit to
    ./scripts/status only, never ./scripts/dev). The direction this test DOES
    cover reliably is an edit to commit_helper.py, which does invalidate
    ./scripts/dev. The other direction is covered by the Go twin,
    TestStructuralGatesAreLiveStages in scripts/status/verify_run_test.go, which
    calls stagesForMode in-process. Keep BOTH; each closes the other's hole.
    """

    def _live_stage_names(self) -> set[str]:
        repo = Path(__file__).resolve().parents[2]
        src_path = repo / "scripts" / "status" / "verify_run.go"
        try:
            src = src_path.read_text(encoding="utf-8")
        except OSError as exc:
            self.fail(
                f"cannot read {src_path} ({exc}) -- did verify_run.go move? "
                "Update _live_stage_names."
            )
        start = src.find("func stagesForMode(")
        if start < 0:
            self.fail(
                f"stagesForMode not found in {src_path} -- renamed or moved? "
                "Update _live_stage_names. This test must not pass vacuously."
            )
        end = src.find("\nfunc ", start + 1)
        if end < 0:
            # stagesForMode is the last function in the file.
            end = len(src)
        body = src[start:end]
        # Strip line comments BEFORE matching. Without this, a commented-out or
        # historical `mk("...")` inside the function body counts as a live stage,
        # which silently re-admits a dead STRUCTURAL_GATES entry -- the exact
        # false negative this test exists to prevent. (No `mk("...")` call in
        # this codebase is preceded by a `//` on the same line, so dropping
        # everything after `//` cannot lose a real one.)
        body = re.sub(r"//.*", "", body)
        return set(re.findall(r'\bmk\("([^"]+)"\)', body))

    def test_structural_gates_are_live_stages(self):
        live = self._live_stage_names()
        # Guard against a parse that silently matched nothing: an empty set
        # would make the subset assertion below pass vacuously.
        self.assertGreater(
            len(live), 10, f"parsed too few stages from stagesForMode: {live}"
        )
        dead = sorted(ch.STRUCTURAL_GATES - live)
        self.assertEqual(
            dead,
            [],
            "STRUCTURAL_GATES names stages that stagesForMode never emits, so "
            f"structural_gate_reds can never match them: {dead}",
        )

    def test_structural_gates_is_not_empty(self):
        # The subset assertion above is satisfied by an empty frozenset too.
        self.assertTrue(ch.STRUCTURAL_GATES)


if __name__ == "__main__":
    unittest.main()
