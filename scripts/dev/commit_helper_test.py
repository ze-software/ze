#!/usr/bin/env python3
"""Unit tests for the discovery-index commit gate in commit_helper.py."""

from __future__ import annotations

import argparse
import contextlib
import io
import os
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest
from pathlib import Path
from unittest import mock

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


# Stands in for a real discovery generator, so it MUST honour the same contract:
# `--root DIR` selects the tree, and a stale output exits STALE_EXIT (3) -- 1 means
# "the generator itself failed" and callers must not read that as drift.
FAKEGEN = textwrap.dedent("""\
    import sys
    from pathlib import Path
    argv = sys.argv
    root = (
        Path(argv[argv.index("--root") + 1]).resolve()
        if "--root" in argv
        else Path(__file__).resolve().parents[2]
    )
    out = root / "ai" / "FAKE.md"
    src = root / "src"
    names = sorted(p.name for p in src.glob("*.txt")) if src.is_dir() else []
    content = ",".join(names)
    if "--check" in argv:
        cur = out.read_text() if out.exists() else ""
        if cur != content:
            print("is stale")
            sys.exit(3)
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


def _seed_fake_index_repo(root: Path) -> None:
    """A repo whose single committed index matches its single committed source."""
    _git(root, "init", "-q")
    _git(root, "config", "user.email", "t@example.com")
    _git(root, "config", "user.name", "t")
    _git(root, "config", "commit.gpgsign", "false")
    (root / "scripts" / "dev").mkdir(parents=True)
    (root / "src").mkdir()
    (root / "ai").mkdir()
    (root / "scripts" / "dev" / "fakegen.py").write_text(FAKEGEN)
    (root / "src" / "a.txt").write_text("a\n")
    (root / "ai" / "FAKE.md").write_text("a.txt")
    _git(root, "add", "-A")
    _git(root, "commit", "-qm", "init")


class TestVerdictOrdering(unittest.TestCase):
    """The HEAD pass must be judged BEFORE the commit overlay is applied.

    Both verdicts come from one materialized tree, so the order is load-bearing:
    overlay first and the commit's own not-yet-committed files would be counted as
    already committed, turning the HEAD verdict into a verdict on nothing (it
    would report the very drift THIS commit is about to introduce as a prior
    commit's bypass, and go quiet about a real one).
    """

    def test_head_verdict_excludes_this_commits_overlay(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_fake_index_repo(root)
            # A new source this commit adds, WITHOUT the matching index update.
            (root / "src" / "b.txt").write_text("b\n")

            saved = (ch.DISCOVERY_INDEX_GENERATORS, ch.DISCOVERY_INDEX_OUTPUTS)
            ch.DISCOVERY_INDEX_GENERATORS = ("scripts/dev/fakegen.py",)
            ch.DISCOVERY_INDEX_OUTPUTS = ("ai/FAKE.md",)
            try:
                v = ch.discovery_index_verdicts(root, ("src/b.txt",), ())
            finally:
                ch.DISCOVERY_INDEX_GENERATORS, ch.DISCOVERY_INDEX_OUTPUTS = saved
            # HEAD itself is coherent: src/b.txt is not committed yet.
            self.assertEqual(v.head_state, "fresh")
            self.assertEqual(v.head_stale, [])
            # The tree this commit PRODUCES is not: the index misses src/b.txt.
            self.assertTrue(v.view_judged)
            self.assertEqual(v.view_stale, ["ai/FAKE.md"])


class TestCommitViewSweep(unittest.TestCase):
    """Leftover commit views are reaped; live ones are never touched.

    Each view is a ~98MB tree plus a ~90MB tar, and a SIGKILL runs no `finally`.
    """

    def test_reaps_only_aged_leftovers(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scratch = root / "tmp"
            scratch.mkdir()
            old_dir = scratch / "commit-view-old"
            old_dir.mkdir()
            (old_dir / "nested").mkdir()
            (old_dir / "nested" / "f.txt").write_text("x")
            old_tar = scratch / "commit-view-old.tar"
            old_tar.write_text("tar bytes")
            fresh_dir = scratch / "commit-view-live"
            fresh_dir.mkdir()
            unrelated = scratch / "commit-abcdef.sh"
            unrelated.write_text("#!/bin/sh\n")

            aged = time.time() - (ch.COMMIT_VIEW_TTL_SECONDS + 60)
            os.utime(old_dir, (aged, aged))
            os.utime(old_tar, (aged, aged))
            os.utime(unrelated, (aged, aged))

            ch._sweep_stale_commit_views(root)

            self.assertFalse(old_dir.exists())  # aged tree: gone
            self.assertFalse(old_tar.exists())  # its tar too
            self.assertTrue(fresh_dir.exists())  # a live run's view: untouched
            self.assertTrue(unrelated.exists())  # not a commit view: untouched

    def test_no_scratch_dir_is_not_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            ch._sweep_stale_commit_views(Path(tmp))  # must not raise


DEFERRALS_HEADER = (
    "| Date | Source | What | Reason | Destination | Status |\n"
    "|------|--------|------|--------|-------------|--------|\n"
)


class TestDeferralDestination(unittest.TestCase):
    """ai/rules/planning.md: an open deferral always names a spec that exists."""

    def _repo(self, tmp: str, rows: str) -> Path:
        root = Path(tmp)
        (root / "plan" / "deferrals").mkdir(parents=True)
        (root / "plan" / "spec-rib-deferred-ipv6-coverage.md").write_text("# Spec\n")
        # Deferrals are sharded per source; the gate folds over plan/deferrals/*.md.
        (root / "plan" / "deferrals" / "spec-rib.md").write_text(
            DEFERRALS_HEADER + rows
        )
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
            (root / "plan" / "deferrals").mkdir(parents=True)
            (root / "plan" / "deferrals" / "spec-rib.md").write_text(
                DEFERRALS_HEADER
                + "| 2026-07-16 | spec-rib.md | old item | time | later maybe | done |\n"
            )
            self.assertEqual(ch.deferral_unassigned_problems(root), [])


def _seed_shard_repo(root: Path, rows: list[tuple[str, str]]) -> None:
    """A committed repo holding one shard whose rows carry the given statuses."""
    _git(root, "init", "-q")
    _git(root, "config", "user.email", "t@example.com")
    _git(root, "config", "user.name", "t")
    _git(root, "config", "commit.gpgsign", "false")
    (root / "plan" / "deferrals").mkdir(parents=True)
    body = "".join(
        f"| 2026-08-03 | spec-gone | what-{i} | reason |"
        f" plan/spec-home.md | {status} |\n"
        for i, (_dest, status) in enumerate(rows)
    )
    (root / "plan" / "spec-home.md").write_text("# Spec\n")
    (root / "plan" / "deferrals" / "spec-gone.md").write_text(DEFERRALS_HEADER + body)
    _git(root, "add", "-A")
    _git(root, "commit", "-qm", "init")


class TestDeferralShardRemoval(unittest.TestCase):
    """`git rm` of a shard is correct ONLY when every row in it is terminal.

    Driven through commit_gate_problems, the BLOCK-severity entry point create()
    actually calls -- not through deferral_shard_removal_problems alone. A gate
    that works when called directly and is never wired into the assembly is the
    failure this drives out (ai/rules/evidence.md, ai/rules/completion.md).
    """

    def test_removing_a_shard_with_a_live_row_is_refused(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "deferred")])
            problems = ch.commit_gate_problems(
                root, (), ("plan/deferrals/spec-gone.md",)
            )
            self.assertTrue(problems, "a live row must block its shard's removal")
            self.assertIn("still holds live rows", problems[0])
            self.assertIn("what-0", problems[0])

    def test_removing_an_all_terminal_shard_is_allowed(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(
                root,
                [("plan/spec-home.md", "done"), ("plan/spec-home.md", "cancelled")],
            )
            self.assertEqual(
                ch.commit_gate_problems(root, (), ("plan/deferrals/spec-gone.md",)),
                [],
                "closure must still be able to remove residue",
            )

    def test_a_live_row_blocks_even_beside_terminal_ones(self) -> None:
        # The all-terminal test above passes if the gate looks only at row 0.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(
                root, [("plan/spec-home.md", "done"), ("plan/spec-home.md", "open")]
            )
            problems = ch.commit_gate_problems(
                root, (), ("plan/deferrals/spec-gone.md",)
            )
            self.assertTrue(problems, "a live row anywhere in the shard must block")
            self.assertIn("what-1", problems[0])
            self.assertNotIn("what-0", problems[0])

    def test_removing_a_spec_does_not_trip_the_shard_gate(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "deferred")])
            self.assertEqual(
                ch.commit_gate_problems(root, (), ("plan/spec-gone.md",)),
                [],
                "the gate is scoped to plan/deferrals/, not every removal",
            )

    def test_deleting_the_working_copy_first_does_not_clear_the_gate(self) -> None:
        # The gate MUST read HEAD, not the working tree. A working-tree read
        # passes every other case here (both trees are identical in them), so
        # without this case `rm`ing the shard before running create --remove is
        # a clean bypass of the whole gate.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "deferred")])
            (root / "plan" / "deferrals" / "spec-gone.md").unlink()
            problems = ch.commit_gate_problems(
                root, (), ("plan/deferrals/spec-gone.md",)
            )
            self.assertTrue(problems, "the live row is still in HEAD, so it blocks")
            self.assertIn("what-0", problems[0])

    def test_a_nested_shard_is_in_scope(self) -> None:
        # deferral_shard_paths globs recursively and deferral_in_diff_problems
        # accepts any depth, so a nested shard that escaped THIS gate could be
        # deleted while the folds that watch it still counted it.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "done")])
            nested = root / "plan" / "deferrals" / "sub" / "spec-deep.md"
            nested.parent.mkdir()
            nested.write_text(
                DEFERRALS_HEADER + "| 2026-08-03 | spec-deep | deep-work | reason |"
                " plan/spec-home.md | deferred |\n"
            )
            _git(root, "add", "-A")
            _git(root, "commit", "-qm", "nested")
            problems = ch.commit_gate_problems(
                root, (), ("plan/deferrals/sub/spec-deep.md",)
            )
            self.assertTrue(problems, "a nested shard must be checked too")
            self.assertIn("deep-work", problems[0])

    def test_renaming_a_live_shard_is_a_move_not_a_deletion(self) -> None:
        # The rows survive at the new path, so nothing is lost. A gate that
        # refused this would freeze a misnamed shard in place -- and a misnamed
        # shard is exactly the one no stem-pairing gate can see.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "deferred")])
            src = (root / "plan" / "deferrals" / "spec-gone.md").read_text()
            (root / "plan" / "deferrals" / "renamed.md").write_text(src)
            self.assertEqual(
                ch.commit_gate_problems(
                    root,
                    ("plan/deferrals/renamed.md",),
                    ("plan/deferrals/spec-gone.md",),
                ),
                [],
            )

    def test_a_partial_move_still_blocks_on_the_rows_left_behind(self) -> None:
        # Copying only SOME rows to the new shard is a deletion of the rest.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(
                root,
                [("plan/spec-home.md", "deferred"), ("plan/spec-home.md", "deferred")],
            )
            lines = (
                (root / "plan" / "deferrals" / "spec-gone.md").read_text().splitlines()
            )
            (root / "plan" / "deferrals" / "renamed.md").write_text(
                "\n".join(lines[:-1]) + "\n"
            )
            problems = ch.commit_gate_problems(
                root,
                ("plan/deferrals/renamed.md",),
                ("plan/deferrals/spec-gone.md",),
            )
            self.assertTrue(problems, "the row left behind must still block")
            self.assertIn("what-1", problems[0])
            self.assertNotIn("what-0", problems[0])

    def test_an_unexpected_git_failure_is_reported(self) -> None:
        # Pins the REPORTED branch against a stderr that mentions HEAD but is
        # none of the benign phrases. Without this, narrowing the benign test to
        # `"HEAD" in stderr` passes every other case and waves corruption through.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "deferred")])
            broken = subprocess.CompletedProcess(
                args=[],
                returncode=128,
                stdout="",
                stderr="fatal: unable to read object for 'HEAD'\n",
            )
            with mock.patch.object(ch.subprocess, "run", return_value=broken):
                problems = ch.deferral_shard_removal_problems(
                    root, (), ("plan/deferrals/spec-gone.md",)
                )
            self.assertTrue(problems, "an unexpected git failure must be reported")
            self.assertIn("cannot be read at HEAD", problems[0])

    def test_index_added_but_uncommitted_shard_is_benign(self) -> None:
        # `exists on disk, but not in 'HEAD'`: staged this commit, never
        # committed, so removing it destroys no committed row.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "done")])
            fresh = root / "plan" / "deferrals" / "spec-fresh.md"
            fresh.write_text(
                DEFERRALS_HEADER + "| 2026-08-03 | spec-fresh | new-work | reason |"
                " plan/spec-home.md | deferred |\n"
            )
            _git(root, "add", "plan/deferrals/spec-fresh.md")
            self.assertEqual(
                ch.commit_gate_problems(root, (), ("plan/deferrals/spec-fresh.md",)),
                [],
            )

    def test_an_unreadable_shard_is_reported_not_waved_through(self) -> None:
        # `git show` failing for a reason OTHER than "absent from HEAD" or
        # "HEAD is unborn" means the gate cannot SEE the rows. A gate that
        # cannot see must say so, never pass (ai/rules/evidence.md).
        # Without this case, `if returncode: continue` reads every breakage --
        # a corrupt object store, git missing, a permission failure -- as
        # "nothing to protect", and the loudest failures become the quietest.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "deferred")])
            shutil.rmtree(root / ".git" / "objects")
            problems = ch.commit_gate_problems(
                root, (), ("plan/deferrals/spec-gone.md",)
            )
            self.assertTrue(problems, "an unreadable shard must not pass silently")
            self.assertIn("cannot be read at HEAD", problems[0])

    def test_an_unborn_head_has_nothing_to_protect(self) -> None:
        # The counterpart: no commit exists, so no committed row can be lost.
        # Reporting here would fire on every fresh repository.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init", "-q")
            self.assertEqual(
                ch.commit_gate_problems(root, (), ("plan/deferrals/spec-gone.md",)),
                [],
            )

    def test_a_shard_new_in_this_commit_is_not_read_from_head(self) -> None:
        # Nothing committed can be destroyed by removing a path HEAD never had,
        # and `git show` on it fails -- the gate must not treat that as a live row.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _seed_shard_repo(root, [("plan/spec-home.md", "done")])
            self.assertEqual(
                ch.commit_gate_problems(root, (), ("plan/deferrals/spec-never.md",)),
                [],
            )


def _deferral_gate(rows: list[tuple[str, str]]) -> list[str]:
    """Run deferral_unassigned_problems over a table of (destination, status).

    Any plan/*.md named by a destination is created, so these cases isolate the
    STATUS half: a row that fails here fails because of its status, never because
    its spec happens not to exist in the fixture. Resolution goes through
    ch.deferral_destination_paths so the harness creates exactly the paths the
    gate checks; resolving it a second time here would let the two drift.

    The rows are written into a single plan/deferrals/ shard; the gate folds over
    every shard in that directory.
    """
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        (root / "plan" / "deferrals").mkdir(parents=True)
        body = ""
        for i, (dest, status) in enumerate(rows):
            for ref in ch.deferral_destination_paths(dest):
                (root / ref).parent.mkdir(parents=True, exist_ok=True)
                (root / ref).write_text("# Spec\n")
            body += f"| 2026-07-16 | spec-x | what-{i} | reason | {dest} | {status} |\n"
        (root / "plan" / "deferrals" / "spec-x.md").write_text(DEFERRALS_HEADER + body)
        return ch.deferral_unassigned_problems(root)


class TestDeferralUnassigned(unittest.TestCase):
    """The STATUS half of the gate enforcing ai/rules/planning.md's "no
    deferral without a destination".

    TestDeferralDestination above covers which Destination cells are a valid home.
    This class covers WHICH ROWS ARE LOOKED AT AT ALL, which is the half that was
    fail-open (ai/rules/evidence.md): the gate tested `status == "open"`,
    so every row at `deferred` bypassed the destination check no matter how strict
    that check became.
    """

    # VALIDATES: a row at status `deferred` with no destination is flagged.
    # `deferred` is the word ai/rules/planning.md itself uses for this
    # state, and most of the rows across plan/deferrals/ carry it.
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
    # A deferral shard really carries `none (permanent exclusion; ...)` at
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
    # was silently treated as absent-and-fine (ai/rules/evidence.md:
    # a guard that neither denies nor speaks does not exist).
    def test_malformed_row_is_reported_not_skipped(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan" / "deferrals").mkdir(parents=True)
            (root / "plan" / "deferrals" / "spec-x.md").write_text(
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
            (root / "plan" / "deferrals").mkdir(parents=True)
            (root / "plan" / "deferrals" / "spec-x.md").write_text(DEFERRALS_HEADER)
            self.assertEqual(ch.deferral_unassigned_problems(root), [])


class TestDeferralGateSeverity(unittest.TestCase):
    """An unhomed deferral is ADVISORY: its message rides commit_gate_warnings,
    never commit_gate_problems, so a bookkeeping-only issue (harmless to software
    behaviour) cannot hard-block a commit (ai/rules/planning.md). The
    detector still flags it, so nothing goes silent.
    """

    def _repo_with_unhomed_row(self, tmp: str) -> Path:
        root = Path(tmp)
        (root / "plan" / "deferrals").mkdir(parents=True)
        (root / "plan" / "deferrals" / "spec-x.md").write_text(
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
    (ai/rules/planning.md, ai/rules/repo-maintenance.md).
    """

    def _repo(self, tmp: str) -> Path:
        root = Path(tmp)
        _git(root, "init")
        _git(root, "config", "user.email", "t@e.st")
        _git(root, "config", "user.name", "t")
        # Disable commit signing for this THROWAWAY fixture repo, matching every
        # sibling commit test in this file. A global commit.gpgsign=true (this
        # repo's owner has one) is inherited here, and gpg cannot reach a tty for
        # a passphrase from a test subprocess, so `git commit` exits 128 and all
        # six TestDeferralInDiff cases error in setUp. Fixture-only: it never
        # touches a real commit, which ai/rules/git-safety.md forbids unsigning.
        _git(root, "config", "commit.gpgsign", "false")
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
            (root / "ai" / "rules" / "planning.md").write_text(
                "# Condensed\n\nRisks: new coupling, follow-up work, future work.\n"
            )
            self.assertEqual(
                ch.deferral_in_diff_problems(root, ("ai/rules/planning.md",), ()), []
            )

    def test_bare_deferral_in_spec_still_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-x.md").write_text(
                "# Spec\n\nThe IPv6 path is future work.\n"
            )
            self.assertTrue(ch.deferral_in_diff_problems(root, ("plan/spec-x.md",), ()))

    # VALIDATES: staging a plan/deferrals/ shard clears the gate -- the deferral
    # was recorded, so the added prose describing it is not un-homed.
    # PREVENTS: a regression to the retired single-file check that only recognised
    # the literal "plan/deferrals.md" path and would still block once it is gone.
    def test_cleared_by_deferrals_shard_in_commit(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "plan" / "deferrals").mkdir(parents=True)
            (root / "plan" / "spec-x.md").write_text(
                "# Spec\n\nThe IPv6 path is future work.\n"
            )
            # The shard MUST exist on disk: `_prospective_added_lines` runs
            # `git add -- <paths>`, which aborts the whole add when any pathspec
            # matches nothing, yielding an empty diff that would clear the gate
            # vacuously (it would return [] even under the retired code). Writing
            # the shard makes the second assertion discriminate: the diff then
            # carries spec-x.md's "future work", and only the new
            # startswith("plan/deferrals/") clear (not the old literal
            # "plan/deferrals.md" check) suppresses it.
            (root / "plan" / "deferrals" / "spec-x.md").write_text(DEFERRALS_HEADER)
            # Without a shard staged, the prose is flagged.
            self.assertTrue(ch.deferral_in_diff_problems(root, ("plan/spec-x.md",), ()))
            # A shard under plan/deferrals/ in the same commit clears it.
            self.assertEqual(
                ch.deferral_in_diff_problems(
                    root, ("plan/spec-x.md", "plan/deferrals/spec-x.md"), ()
                ),
                [],
            )

    # PREVENTS a too-loose match: a path that merely starts with "plan/deferrals"
    # but is NOT inside the directory (e.g. a stray "plan/deferrals-notes.md")
    # must not clear the gate. Only the directory counts.
    def test_not_cleared_by_lookalike_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-x.md").write_text(
                "# Spec\n\nThe IPv6 path is future work.\n"
            )
            # A real file whose path only LOOKS like the shard directory. It must
            # not clear the gate, and it must exist so git can stage it.
            (root / "plan" / "deferrals-notes.md").write_text("# notes\n\nplain text\n")
            self.assertTrue(
                ch.deferral_in_diff_problems(
                    root, ("plan/spec-x.md", "plan/deferrals-notes.md"), ()
                )
            )


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
    does not. A remediation that cannot work is worse than none (ai/rules/cli.md).
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


class TestStructuralRedOwnerOverride(unittest.TestCase):
    """The structural-gate refusal needs ONE escape, and only the owner may use it.

    VALIDATES: `ai/rules/git-safety.md` "Structural Gates Are Never Known-Red" --
    a structural red is never flaky, so `--unverified` and a
    `plan/known-failures/` shard must keep failing to bypass it. But the refusal
    had no override at all, which made a green tree the only route to any commit,
    including one that touches no compiled code. `--structural-red-ok "<reason>"`
    is that route, kept separate from `--unverified` so it can never be reached by
    the flaky-test path, and required to carry a reason.
    PREVENTS: an agent silently widening `--unverified` (or editing
    STRUCTURAL_GATES) to get past a red tree, which is the hole the refusal was
    added to close.
    """

    def _run(self, extra: list[str]):
        import contextlib
        import io

        saved = (ch.verify_status, ch.structural_gate_reds)
        ch.verify_status = lambda repo: ("stale", "structural red")
        ch.structural_gate_reds = lambda repo: ["ze-regen-check-readonly"]
        try:
            with tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                _git(root, "init", "-q")
                _git(root, "config", "user.email", "t@example.com")
                _git(root, "config", "user.name", "t")
                _git(root, "config", "commit.gpgsign", "false")
                (root / "f.txt").write_text("hello\n")
                err = io.StringIO()
                out = io.StringIO()
                with contextlib.redirect_stderr(err), contextlib.redirect_stdout(out):
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
                            "fixture test for the structural-red owner override",
                        ]
                        + extra
                    )
                return rc, err.getvalue() + out.getvalue()
        finally:
            ch.verify_status, ch.structural_gate_reds = saved

    def test_without_the_flag_it_still_refuses(self):
        rc, msg = self._run([])
        self.assertEqual(rc, 2, msg)
        self.assertIn("STRUCTURAL GATE", msg)

    def test_unverified_alone_still_cannot_bypass(self):
        rc, msg = self._run(["--unverified", "flaky ospf timeout elsewhere"])
        self.assertEqual(rc, 2, msg)
        self.assertIn("STRUCTURAL GATE", msg)

    def test_owner_override_allows_the_commit_and_names_the_red_gate(self):
        rc, msg = self._run(
            ["--structural-red-ok", "owner: docs-only commit, red is another session"]
        )
        self.assertEqual(rc, 0, msg)
        # The override must be LOUD: silently proceeding would make a red tree
        # indistinguishable from a green one in the session transcript.
        self.assertIn("ze-regen-check-readonly", msg)

    def test_override_requires_a_reason(self):
        rc, msg = self._run(["--structural-red-ok", "   "])
        self.assertEqual(rc, 2, msg)


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
    prevent (ai/rules/evidence.md). That is the opposite choice from
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


class TestLearnedNextCounterFree(unittest.TestCase):
    """learned_next allocates from max(glob)+1 alone, with no plan/learned/.counter.

    The .counter cache is retired: the NNNN-slug.md filenames ARE the record,
    so allocation is max(existing prefixes) + 1 and the O_EXCL create is the
    only same-tree mutual exclusion. A losing racer must retry onto the next
    free number, never surface an uncaught FileExistsError.
    """

    def _repo(self, tmp: str) -> Path:
        root = Path(tmp)
        _git(root, "init", "-q")
        return ch.repo_root(str(root))

    def test_learned_next_unique_without_counter(self):
        # AC-4: two allocations in one tree with no .counter present yield
        # distinct numbers, relying solely on the O_EXCL create + max(glob).
        with tempfile.TemporaryDirectory() as tmp:
            repo = self._repo(tmp)
            learned = repo / "plan" / "learned"
            learned.mkdir(parents=True)
            (learned / "500-seed.md").write_text("# 500 -- seed\n")

            self.assertEqual(
                ch.learned_next(argparse.Namespace(repo=str(repo), slug="alpha")), 0
            )
            self.assertEqual(
                ch.learned_next(argparse.Namespace(repo=str(repo), slug="beta")), 0
            )

            nums = sorted(
                int(p.name.split("-", 1)[0]) for p in learned.glob("[0-9]*-*.md")
            )
            self.assertEqual(nums, [500, 501, 502])
            self.assertFalse((learned / ".counter").exists())

    def test_learned_next_retries_on_existing(self):
        # AC-4/R-6: simulate the same-tree race -- a concurrent session wins
        # the target number in the window between this session's glob and its
        # O_EXCL create. Today the uncaught FileExistsError escapes learned_next
        # (main() catches only UsageError); the bounded retry must re-glob past
        # the winner and land on the next free number instead of crashing.
        with tempfile.TemporaryDirectory() as tmp:
            repo = self._repo(tmp)
            learned = repo / "plan" / "learned"
            learned.mkdir(parents=True)
            (learned / "500-seed.md").write_text("# 500 -- seed\n")

            real_open = os.open
            first = {"pending": True}

            def racing_open(path, *a, **k):
                # First exclusive create of a summary: a concurrent session
                # wins that number (create the file), then fail this caller's
                # O_EXCL exactly as the kernel would.
                if first["pending"] and str(path).endswith(".md"):
                    first["pending"] = False
                    os.close(
                        real_open(
                            str(path), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o644
                        )
                    )
                    raise FileExistsError(17, "File exists", str(path))
                return real_open(path, *a, **k)

            args = argparse.Namespace(repo=str(repo), slug="alpha")
            with mock.patch.object(ch.os, "open", side_effect=racing_open):
                rc = ch.learned_next(args)

            self.assertEqual(rc, 0)
            nums = sorted(
                int(p.name.split("-", 1)[0]) for p in learned.glob("[0-9]*-*.md")
            )
            # 500 seed, 501 the racing winner, 502 this session after the retry.
            self.assertEqual(nums, [500, 501, 502])
            self.assertFalse((learned / ".counter").exists())


class TestDeferralSharding(unittest.TestCase):
    """AC-1: deferrals are sharded one file per source under plan/deferrals/, so a
    session stages only files it owns and git merges disjoint creations without
    conflict (ai/rules/planning.md, ai/rules/git-safety.md). The bug this
    fixes: a single shared plan/deferrals.md means `git add <file>` stages every
    session's pending rows, so whoever commits first carries the others'.
    """

    def _commit_repo(self, tmp: str) -> Path:
        root = Path(tmp)
        _git(root, "init", "-q")
        _git(root, "config", "user.email", "t@e.st")
        _git(root, "config", "user.name", "t")
        _git(root, "config", "commit.gpgsign", "false")
        (root / "seed.txt").write_text("seed\n")
        _git(root, "add", "seed.txt")
        _git(root, "commit", "-q", "-m", "seed")
        return root

    def _committed_files(self, root: Path) -> list[str]:
        out = subprocess.run(
            ["git", "show", "--name-only", "--pretty=format:", "HEAD"],
            cwd=root,
            check=True,
            capture_output=True,
            text=True,
        ).stdout
        return [ln for ln in out.splitlines() if ln.strip()]

    # VALIDATES AC-1: two sessions each write a DIFFERENT deferral shard; each
    # stages only its own shard, so neither commit carries the other's row.
    def test_two_sessions_no_cross_commit(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._commit_repo(tmp)
            (root / "plan" / "deferrals").mkdir(parents=True)
            a = "plan/deferrals/spec-a.md"
            b = "plan/deferrals/spec-b.md"
            (root / a).write_text(
                DEFERRALS_HEADER
                + "| 2026-07-21 | spec-a | work A | reason | `plan/spec-a.md` | deferred |\n"
            )
            (root / b).write_text(
                DEFERRALS_HEADER
                + "| 2026-07-21 | spec-b | work B | reason | `plan/spec-b.md` | deferred |\n"
            )
            # Both shards are dirty in the same working tree. Session A stages and
            # commits ONLY its own shard.
            _git(root, "add", a)
            _git(root, "commit", "-q", "-m", "session A deferral")
            files_a = self._committed_files(root)
            self.assertIn(a, files_a)
            self.assertNotIn(b, files_a)

            # Session B's shard is still uncommitted; it commits independently.
            _git(root, "add", b)
            _git(root, "commit", "-q", "-m", "session B deferral")
            files_b = self._committed_files(root)
            self.assertIn(b, files_b)
            self.assertNotIn(a, files_b)

    # PREVENTS a vacuous pass: proves the isolation is REAL by contrast. Two rows
    # in ONE shared file behave the old, broken way -- `git add <file>` stages the
    # whole file, so a single commit carries BOTH sessions' rows. This is exactly
    # the cross-commit the per-source sharding removes.
    def test_single_file_would_cross_commit(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._commit_repo(tmp)
            (root / "plan").mkdir()
            shared = "plan/deferrals-single.md"
            (root / shared).write_text(
                DEFERRALS_HEADER
                + "| 2026-07-21 | spec-a | work A | reason | `plan/spec-a.md` | deferred |\n"
                + "| 2026-07-21 | spec-b | work B | reason | `plan/spec-b.md` | deferred |\n"
            )
            _git(root, "add", shared)
            _git(root, "commit", "-q", "-m", "session A commits the shared file")
            body = subprocess.run(
                ["git", "show", "HEAD:" + shared],
                cwd=root,
                check=True,
                capture_output=True,
                text=True,
            ).stdout
            # Session A's commit carried session B's row -- the defect.
            self.assertIn("work A", body)
            self.assertIn("work B", body)

    # VALIDATES: deferral_unassigned_problems folds over EVERY shard, so an unhomed
    # row in one shard is surfaced even when other shards are clean.
    def test_unassigned_folds_across_shards(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan" / "deferrals").mkdir(parents=True)
            (root / "plan" / "spec-real.md").write_text("# Spec\n")
            (root / "plan" / "deferrals" / "clean.md").write_text(
                DEFERRALS_HEADER
                + "| 2026-07-21 | spec-real | ok | reason | `plan/spec-real.md` | deferred |\n"
            )
            (root / "plan" / "deferrals" / "unhomed.md").write_text(
                DEFERRALS_HEADER
                + "| 2026-07-21 | spec-x | dropped | reason | future work | deferred |\n"
            )
            problems = ch.deferral_unassigned_problems(root)
            self.assertTrue(problems, "an unhomed row in any shard must be surfaced")
            joined = "\n".join(problems)
            self.assertIn("dropped", joined)
            self.assertNotIn("| ok |", joined)

    # VALIDATES: the fold is RECURSIVE, matching deferral_in_diff's any-depth
    # clearing (startswith "plan/deferrals/"). A shard nested in a subdirectory
    # that would CLEAR the block gate must also be CHECKED by the advisory fold,
    # or a nested shard could clear the block while escaping the unassigned check.
    def test_subdir_shard_is_folded(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan" / "deferrals" / "nested").mkdir(parents=True)
            (root / "plan" / "deferrals" / "nested" / "unhomed.md").write_text(
                DEFERRALS_HEADER
                + "| 2026-07-21 | spec-x | nested dropped | reason | future work | deferred |\n"
            )
            paths = [p.name for p in ch.deferral_shard_paths(root)]
            self.assertIn("unhomed.md", paths, "rglob must reach a nested shard")
            problems = ch.deferral_unassigned_problems(root)
            self.assertTrue(problems, "a nested unhomed row must be surfaced")
            self.assertIn("nested dropped", "\n".join(problems))


PCV_HEADER = "## Pre-Commit Verification\n\n"
PCV_TABLES = (
    "### Files Exist (ls)\n"
    "| File | Exists | Evidence |\n"
    "|------|--------|----------|\n"
    "{files}"
    "\n### AC Verified (grep/test)\n"
    "| AC ID | Claim | Fresh Evidence |\n"
    "|-------|-------|----------------|\n"
    "{ac}"
    "\n### Wiring Verified (end-to-end)\n"
    "| Entry Point | .ci File | Verified |\n"
    "|-------------|----------|----------|\n"
    "{wiring}"
)


def _pcv(files: str = "", ac: str = "", wiring: str = "") -> str:
    return PCV_HEADER + PCV_TABLES.format(files=files, ac=ac, wiring=wiring)


class TestPreCommitVerificationFilled(unittest.TestCase):
    """The closure gate must check EVERY evidence sub-table, not the section.

    VALIDATES: `ai/rules/completion.md` Pre-Commit Verification --
    "For each item: run a command and paste the evidence". Each sub-table is a
    separate obligation (files exist / AC re-verified / wiring re-read), so
    evidence for one is not evidence for another.
    PREVENTS: the measured failure mode -- one row in `Files Exist` satisfying
    the gate while `AC Verified` and `Wiring Verified` stay empty. Across the
    in-progress specs on 2026-07-25 that left ~73% of `AC Verified` and ~75% of
    `Wiring Verified` byte-identical to the template at closure.
    """

    def test_absent_section_is_none(self):
        self.assertIsNone(ch.pre_commit_verification_gaps("## Task\n\nbody\n"))

    def test_every_subtable_filled_passes(self):
        spec = _pcv(
            files="| `a.go` | yes | ls output |\n",
            ac="| AC-1 | parses | TestParse pass |\n",
            wiring="| config | `test/parse/a.ci` | read, covers path |\n",
        )
        self.assertEqual(ch.pre_commit_verification_gaps(spec), [])

    def test_one_filled_subtable_does_not_satisfy_the_others(self):
        spec = _pcv(files="| `a.go` | yes | ls output |\n")
        gaps = ch.pre_commit_verification_gaps(spec)
        self.assertIn("AC Verified", " ".join(gaps))
        self.assertIn("Wiring Verified", " ".join(gaps))
        self.assertNotIn("Files Exist", " ".join(gaps))

    def test_separator_only_table_is_not_evidence(self):
        gaps = ch.pre_commit_verification_gaps(_pcv())
        self.assertEqual(len(gaps), 3, gaps)

    # A spec with no sub-headings (pre-2026-07 shape) keeps the old floor:
    # at least one data row somewhere in the section. Widening the gate must not
    # retroactively block a spec whose section never had sub-tables.
    def test_flat_section_falls_back_to_section_level_rule(self):
        flat = "## Pre-Commit Verification\n\n| File | Evidence |\n|---|---|\n"
        self.assertTrue(ch.pre_commit_verification_gaps(flat))
        flat_filled = flat + "| `a.go` | ls output |\n"
        self.assertEqual(ch.pre_commit_verification_gaps(flat_filled), [])

    def test_spec_audit_reports_the_unfilled_subtables_by_name(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan" / "learned").mkdir(parents=True)
            (root / "plan" / "spec-demo.md").write_text(
                _pcv(files="| `a.go` | yes | ls output |\n")
            )
            problems = ch.spec_audit_problems(
                root, ("plan/learned/1200-demo.md",), "spec-demo.md"
            )
            self.assertTrue(problems)
            self.assertIn("AC Verified", problems[0])
            self.assertIn("Wiring Verified", problems[0])


class TestSTEGate(unittest.TestCase):
    """The ASD-STE100 prose gate (ai/rules/writing.md).

    It BLOCKS a commit whose own prose grew one of the six banned habits, and it
    compares each file with its own HEAD version so legacy prose in a file you
    touched costs nothing. Both halves are asserted here: a gate that only ever
    passes, and a gate that fires on inherited text, are equally useless.
    """

    def _repo(self, tmp: str) -> Path:
        """A throwaway git repo carrying a real copy of the checker."""
        root = Path(tmp)
        (root / "scripts" / "dev").mkdir(parents=True)
        (root / "docs").mkdir()
        checker = Path(__file__).with_name("ste_check.py")
        (root / "scripts" / "dev" / "ste_check.py").write_text(
            checker.read_text(encoding="utf-8"), encoding="utf-8"
        )
        _git(root, "init", "-q")
        _git(root, "config", "user.email", "t@example.com")
        _git(root, "config", "user.name", "T")
        return root

    def _commit_all(self, root: Path) -> None:
        _git(root, "add", "-A")
        _git(root, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "base")

    def test_clean_prose_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "x.md").write_text("# X\n\nZe starts the daemon.\n")
            self._commit_all(root)
            (root / "docs" / "x.md").write_text(
                "# X\n\nZe starts the daemon.\n\nZe stops the daemon.\n"
            )
            self.assertEqual(ch.ste_problems(root, ("docs/x.md",)), [])

    def _ste_report(self, root, paths):
        """STE is a guideline: ste_problems REPORTS to stderr and never blocks.
        Returns the report text, and asserts the commit was not refused."""
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            problems = ch.ste_problems(root, paths)
        self.assertEqual(problems, [], "STE is advisory and must never block a commit")
        return err.getvalue()

    def test_new_habit_is_reported_and_named(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "x.md").write_text("# X\n\nZe starts the daemon.\n")
            self._commit_all(root)
            (root / "docs" / "x.md").write_text(
                "# X\n\nZe starts the daemon.\n\nIt should spin up seamlessly.\n"
            )
            report = self._ste_report(root, ("docs/x.md",))
            self.assertIn("docs/x.md", report)
            self.assertIn("hedging", report)
            self.assertIn("phrasal-verbs", report)
            self.assertIn("marketing-adjectives", report)
            self.assertIn("guideline", report)

    def test_inherited_prose_does_not_block(self):
        # The whole point of comparing against HEAD: touching a file that already
        # holds legacy habits must not fail. Only what you added counts.
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            legacy = "# X\n\nIt should spin up seamlessly.\n"
            (root / "docs" / "x.md").write_text(legacy)
            self._commit_all(root)
            (root / "docs" / "x.md").write_text(legacy + "\nZe stops the daemon.\n")
            self.assertEqual(ch.ste_problems(root, ("docs/x.md",)), [])

    def test_non_prose_paths_are_skipped(self):
        # Non-vacuous: the offending prose is IN the non-prose file, so a gate
        # that stopped filtering by suffix would report it.
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "Makefile").write_text("# it should spin up seamlessly\n")
            self._commit_all(root)
            (root / "Makefile").write_text(
                "# it should spin up seamlessly\n# and it should figure out why\n"
            )
            self.assertEqual(ch.ste_problems(root, ("Makefile",)), [])

    def test_dot_directory_path_is_read(self):
        # `str.lstrip("./")` strips a character SET, so ".claude/rules/x.md"
        # became "claude/rules/x.md" and 20 tracked files were silently ungated.
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / ".claude" / "rules").mkdir(parents=True)
            (root / ".claude" / "rules" / "x.md").write_text("# X\n\nZe starts.\n")
            self._commit_all(root)
            (root / ".claude" / "rules" / "x.md").write_text(
                "# X\n\nZe starts.\n\nIt should spin up seamlessly.\n"
            )
            report = self._ste_report(root, (".claude/rules/x.md",))
            self.assertIn(
                ".claude/rules/x.md", report, "a dot-directory path must still be read"
            )

    def test_dot_slash_prefix_is_read(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "x.md").write_text("# X\n\nZe starts.\n")
            self._commit_all(root)
            (root / "docs" / "x.md").write_text(
                "# X\n\nIt should spin up seamlessly.\n"
            )
            self.assertIn("docs/x.md", self._ste_report(root, ("./docs/x.md",)))

    def test_missing_checker_never_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "x.md").write_text("It should spin up seamlessly.\n")
            self.assertEqual(ch.ste_problems(root, ("docs/x.md",)), [])

    def test_a_crashing_checker_fails_open_but_says_so(self):
        # Fail-open is the right verdict, and silence is not. A checker
        # regression must not remove the guard from every commit unannounced.
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "x.md").write_text("# X\n\nZe starts.\n")
            self._commit_all(root)
            (root / "scripts" / "dev" / "ste_check.py").write_text(
                "import sys\nsys.exit(1)\n"
            )
            buf = io.StringIO()
            with contextlib.redirect_stderr(buf):
                problems = ch.ste_problems(root, ("docs/x.md",))
            self.assertEqual(problems, [], "a broken checker must not wedge commits")
            self.assertIn("UNCHECKED", buf.getvalue())

    def test_usage_error_is_not_read_as_a_finding(self):
        # argparse exits 2. The gate's own "habit grew" code is 3, so a
        # malformed invocation cannot surface as prose findings.
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "x.md").write_text("# X\n\nZe starts.\n")
            self._commit_all(root)
            (root / "scripts" / "dev" / "ste_check.py").write_text(
                "import sys\nprint('usage: ...', file=sys.stderr)\nsys.exit(2)\n"
            )
            buf = io.StringIO()
            with contextlib.redirect_stderr(buf):
                self.assertEqual(ch.ste_problems(root, ("docs/x.md",)), [])
            self.assertIn("UNCHECKED", buf.getvalue())


class TestLessonIsContentDriven(unittest.TestCase):
    """The automatic learned-summary demand reads WHAT changed, not WHICH
    directory changed. A commit that only relocates content in a lesson-scoped
    path teaches nothing and is accepted without a summary; one that adds content
    is refused until a summary or an explicit reason arrives
    (plan/spec-knowledge-1-corpus.md AC-1, AC-2).
    """

    def _repo(self, tmp: str) -> Path:
        root = Path(tmp)
        _git(root, "init")
        _git(root, "config", "user.email", "t@e.st")
        _git(root, "config", "user.name", "t")
        # Fixture-only, matching every sibling commit test here: a global
        # commit.gpgsign=true would make `git commit` exit 128 with no tty.
        _git(root, "config", "commit.gpgsign", "false")
        (root / "scripts" / "dev").mkdir(parents=True)
        (root / "ai" / "rules").mkdir(parents=True)
        (root / "scripts" / "dev" / "a.py").write_text(
            "def widen(value):\n    return value * 2\n"
        )
        (root / "ai" / "rules" / "x.md").write_text("# X\n\nThe gate refuses.\n")
        _git(root, "add", "-A")
        _git(root, "commit", "-q", "-m", "seed")
        return root

    def _comment(self, root: Path, paths: tuple[str, ...]) -> str:
        return ch.lesson_comment(
            paths, (), False, None, ch.lesson_change_lines(root, paths, ())
        )

    # VALIDATES: AC-1 -- a closure whose work produced no reusable lesson is
    # accepted with no plan/learned/NNN-*.md staged.
    # PREVENTS: the path-prefix demand returning, which made a summary an
    # unconditional artifact of touching scripts/dev/ and produced 229 summaries
    # with no gotcha.
    def test_lesson_optional_without_content(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            # A pure move: the function leaves a.py and arrives in b.py intact.
            # The fixture used to re-indent the body during the move and still
            # expect "mechanical". It no longer may: in Python the indent IS the
            # block, so a moved-and-re-indented body is a relocation plus an
            # edit. test_reindent_is_not_mechanical pins the other half.
            (root / "scripts" / "dev" / "a.py").write_text("")
            (root / "scripts" / "dev" / "b.py").write_text(
                "def widen(value):\n    return value * 2\n"
            )
            comment = self._comment(root, ("scripts/dev/a.py", "scripts/dev/b.py"))
            self.assertIn("not needed", comment)
            self.assertIn("move or a reformat", comment)

    # VALIDATES: a lesson ROUTED to where it governs behaviour satisfies the
    # demand exactly as a summary does (plan/spec-knowledge-routing.md).
    # PREVENTS: the gate producing archive instead of guidance. Measured
    # 2026-08-03 over 903 summaries, 13 were referenced by a rule or a hook.
    def test_route_satisfies_the_demand(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "architecture").mkdir(parents=True)
            (root / "scripts" / "dev" / "a.py").write_text(
                "def widen(value):\n    return value * 3\n\ndef added():\n    pass\n"
            )
            (root / "docs" / "architecture" / "widen.md").write_text(
                "# Widen\n\nWhy the factor is 3.\n"
            )
            comment = self._comment(
                root, ("scripts/dev/a.py", "docs/architecture/widen.md")
            )
            self.assertIn("routed to", comment)
            self.assertIn("docs/architecture/widen.md", comment)

    # VALIDATES: a destination that is ALSO in the lesson-worthy scope cannot
    # satisfy the demand on its own.
    # PREVENTS: `ai/rules/` self-satisfying, which would turn every rule commit
    # into "never ask" -- the same degradation test_lesson_demanded_with_content
    # guards from the other side.
    def test_route_that_is_the_whole_scope_does_not_satisfy(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "ai" / "rules" / "x.md").write_text(
                "# X\n\nThe gate refuses.\n\nIt now also refuses a self-route.\n"
            )
            with self.assertRaises(ch.UsageError) as caught:
                self._comment(root, ("ai/rules/x.md",))
            self.assertIn("routed", str(caught.exception).lower())

    # VALIDATES: --lesson-required outranks a route, so the operator can still
    # demand the summary itself.
    def test_lesson_required_outranks_a_route(self):
        with self.assertRaises(ch.UsageError):
            ch.lesson_comment(
                ("scripts/dev/a.py", "docs/architecture/widen.md"),
                (),
                True,
                None,
                (("scripts/dev/a.py", "+", "a genuinely new line of content"),),
            )

    # VALIDATES: AC-2 -- a closure whose work produced a lesson is still refused
    # without one, and the refusal names the content that earned the demand.
    # PREVENTS: the content test degrading into "never ask", which would make the
    # summary corpus stop growing for the wrong reason.
    def test_lesson_demanded_with_content(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "ai" / "rules" / "x.md").write_text(
                "# X\n\nThe gate refuses.\n\nIt now also refuses an empty set.\n"
            )
            with self.assertRaises(ch.UsageError) as caught:
                self._comment(root, ("ai/rules/x.md",))
            message = str(caught.exception)
            self.assertIn("adds content", message)
            self.assertIn("refuses an empty set", message)  # the evidence line
            self.assertIn("--lesson-not-needed", message)  # the next step
            # The stated escape actually works.
            self.assertIn(
                "not needed",
                ch.lesson_comment(
                    ("ai/rules/x.md",),
                    (),
                    False,
                    "restates an existing gate for readers",
                    ch.lesson_change_lines(root, ("ai/rules/x.md",), ()),
                ),
            )

    # Boundary for MIN_SUBSTITUTION_SITES (2): one swapped token is an edit and is
    # demanded; the same swap repeated is a mechanical substitution and is not.
    def test_substitution_boundary_is_two_sites(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            path = root / "scripts" / "dev" / "a.py"
            path.write_text("def widen(value):\n    return value * 3\n")
            with self.assertRaises(ch.UsageError):
                self._comment(root, ("scripts/dev/a.py",))
            path.write_text("def widen(amount):\n    return amount * 2\n")
            self.assertIn("substitution", self._comment(root, ("scripts/dev/a.py",)))

    # The two-commit closure survives a spec with no lesson: commit B removes the
    # spec and adds nothing, so it is never asked for a summary it cannot have
    # (ai/rules/planning.md "Spec Closure").
    def test_spec_closure_commit_needs_no_lesson(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-x.md").write_text("# Spec\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-q", "-m", "spec")
            removed = ("plan/spec-x.md",)
            self.assertEqual(ch.lesson_change_lines(root, (), removed), ())
            self.assertIn(
                "not required",
                ch.lesson_comment(
                    (), removed, False, None, ch.lesson_change_lines(root, (), removed)
                ),
            )

    def _kind(self, before: str, after: str) -> str | None:
        """`_mechanical_kind` for a one-file edit from `before` to `after`."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            path = root / "scripts" / "dev" / "a.py"
            path.write_text(before)
            _git(root, "add", "-A")
            _git(root, "commit", "-q", "-m", "before")
            path.write_text(after)
            changes = ch.lesson_change_lines(root, ("scripts/dev/a.py",), ())
            return ch._mechanical_kind(changes)

    # VALIDATES: reordering tokens within a line is not mechanical, so the
    # learned-summary demand is not skipped for it.
    # PREVENTS: the multiset comparison returning. It compared WHICH words the
    # diff added against which it removed, so every one of these read as "a move
    # or a reformat" and the demand was silently waived. The first case is a
    # logic inversion, and it is the exact edit that would disable the staleness
    # ratchet shipped alongside this gate.
    def test_reordering_within_a_line_is_not_mechanical(self):
        for name, before, after in (
            (
                "comparison inverted",
                "def gate(count, baseline):\n    if count > baseline:\n        fail()\n",
                "def gate(count, baseline):\n    if baseline > count:\n        fail()\n",
            ),
            (
                "arguments swapped",
                "def go(src, dst):\n    move(src, dst)\n",
                "def go(src, dst):\n    move(dst, src)\n",
            ),
            (
                "guard order swapped",
                "def use(p):\n    if exists(p) and safe(p):\n        read(p)\n",
                "def use(p):\n    if safe(p) and exists(p):\n        read(p)\n",
            ),
        ):
            with self.subTest(name):
                self.assertIsNone(
                    self._kind(before, after),
                    f"{name}: same words, different meaning -- not mechanical",
                )

    # VALIDATES: swapping two whole LINES is not mechanical. Every word survives
    # and every indent survives, so only the ORDER of the changed lines carries
    # the change.
    # PREVENTS: `_mechanical_kind` comparing the two sequences order-free.
    # Replacing its `shapes_added == shapes_removed` with a sorted() comparison
    # left every other case in this file green while waiving the demand for a
    # reordering -- the docstring claimed an ordered SEQUENCE and nothing held it
    # to that. Acquiring the lock after the work it guards is that shape.
    #
    # The fixture keeps `log(...)` between the two swapped statements on purpose:
    # git minimises an ADJACENT swap to one removed line and one added line, so
    # the swap never reaches this function to be judged.
    def test_swapping_whole_lines_is_not_mechanical(self):
        self.assertIsNone(
            self._kind(
                "def run(lock):\n    lock.acquire()\n    log('start')\n    do_work()\n",
                "def run(lock):\n    do_work()\n    log('start')\n    lock.acquire()\n",
            ),
            "the lock is now taken after the work it guards -- not a relocation",
        )

    # VALIDATES: a re-indent is not mechanical either.
    # PREVENTS: the docstring's old claim that re-indenting "changes no words"
    # being treated as a claim that it changes no MEANING. Dedenting this return
    # out of the loop makes the function return after one item, and scripts/dev/
    # is inside LESSON_WORTHY_PREFIXES.
    def test_reindent_is_not_mechanical(self):
        self.assertIsNone(
            self._kind(
                "def total(items):\n"
                "    n = 0\n"
                "    for i in items:\n"
                "        n += i\n"
                "        return n\n",
                "def total(items):\n"
                "    n = 0\n"
                "    for i in items:\n"
                "        n += i\n"
                "    return n\n",
            ),
            "dedenting a return out of a loop body changes what it returns",
        )

    # The demand's evidence must name the line that CHANGED. Pointing at a
    # removed line to explain a reordering sends the reader to the wrong end of
    # the diff (ai/rules/cli.md: the value, not just the operation).
    def test_reordering_evidence_names_the_added_line(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            path = root / "scripts" / "dev" / "a.py"
            path.write_text("def go(src, dst):\n    move(src, dst)\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-q", "-m", "before")
            path.write_text("def go(src, dst):\n    move(dst, src)\n")
            with self.assertRaises(ch.UsageError) as caught:
                self._comment(root, ("scripts/dev/a.py",))
            message = str(caught.exception)
            self.assertIn("move(dst, src)", message)
            self.assertIn("reordered", message)

    # --lesson-required is the operator saying so. Content never overrides it.
    def test_lesson_required_still_raises_on_mechanical_change(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "scripts" / "dev" / "a.py").write_text("")
            (root / "scripts" / "dev" / "b.py").write_text(
                "def widen(value):\n    return value * 2\n"
            )
            paths = ("scripts/dev/a.py", "scripts/dev/b.py")
            with self.assertRaises(ch.UsageError) as caught:
                ch.lesson_comment(
                    paths, (), True, None, ch.lesson_change_lines(root, paths, ())
                )
            self.assertIn("--lesson-required", str(caught.exception))


if __name__ == "__main__":
    unittest.main()
