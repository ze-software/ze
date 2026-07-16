#!/usr/bin/env python3
"""Unit tests for the discovery-index commit gate in commit_helper.py."""

from __future__ import annotations

import os
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

    def test_placeholder_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp, self._row("-"))
            problems = ch.deferral_unassigned_problems(root)
            self.assertEqual(len(problems), 1)
            self.assertIn("no destination", problems[0])

    def test_prose_destination_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp, self._row("future work, once the RIB settles"))
            problems = ch.deferral_unassigned_problems(root)
            self.assertEqual(len(problems), 1)
            self.assertIn("names no file", problems[0])

    def test_missing_spec_file_blocks(self):
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


if __name__ == "__main__":
    unittest.main()
