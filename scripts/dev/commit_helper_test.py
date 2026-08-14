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
            self.assertFalse(f(root, "plan/learned/1099-z.md"))
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
                tmp,
                self._row(
                    "`plan/spec-rib-deferred-ipv6-coverage.md`"  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
                ),
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
                tmp,
                self._row(
                    "`plan/spec-rib-deferred-nobody-wrote-it.md`"  # <!-- doc-links: ignore (negative fixture: the destination is deliberately absent) -->
                ),
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
                    "`plan/spec-rib-deferred-ipv6-coverage.md` (re-homed 2026-07-16: "  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
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
            root = self._repo(
                tmp, self._row("`plan/learned/1127-rib-arch-2.md`")  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
            )
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
                    (
                        "`plan/spec-fixit-x.md` (F4)",  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
                        "open",
                    ),
                    (
                        "`plan/learned/1127-rib-arch-2.md`",  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
                        "in-progress (spec)",
                    ),
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

    def test_deferral_language_in_vendor_is_exempt(self):
        """A TODO in a dependency is its author's note, not a Ze deferral.

        github.com/andybalholm/brotli carries a "TODO: Postpone decision"
        comment, which blocked the commit that vendored templ. No
        plan/deferrals/ shard could sensibly record another project's comment.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "vendor" / "github.com" / "x").mkdir(parents=True)
            (root / "vendor" / "github.com" / "x" / "encode.go").write_text(
                "package x\n/* TODO: Postpone decision until next block arrives? */\n"
            )
            self.assertEqual(
                ch.deferral_in_diff_problems(
                    root, ("vendor/github.com/x/encode.go",), ()
                ),
                [],
            )

    def test_deferral_language_outside_vendor_still_caught(self):
        """The exemption keys on the vendor/ prefix and nothing wider."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "internal" / "vendored").mkdir(parents=True)
            (root / "internal" / "vendored" / "y.go").write_text(
                "package y\n// TODO: Postpone decision until next block arrives?\n"
            )
            self.assertTrue(
                ch.deferral_in_diff_problems(root, ("internal/vendored/y.go",), ())
            )

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
                # This commit feeds ONLY DOCS-TO-CODE and includes it.
                go_path = root / "internal" / "x" / "y.go"
                go_path.parent.mkdir(parents=True, exist_ok=True)
                go_path.write_text("// Design: docs/x.md\npackage x\n")
                (root / "ai" / "DOCS-TO-CODE.md").write_text("v2\n")
                # PACKAGE-MAP.md is dirty from another session but this commit
                # does not feed it, so the gate must not demand it.
                (root / "ai" / "PACKAGE-MAP.md").write_text("someone-else\n")
                problems = ch.discovery_index_problems(
                    root,
                    ("internal/x/y.go", "ai/DOCS-TO-CODE.md"),
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
                # Commit feeds DOCS-TO-CODE, whose output IS dirty but omitted.
                go_path = root / "internal" / "x" / "y.go"
                go_path.parent.mkdir(parents=True, exist_ok=True)
                go_path.write_text("// Design: docs/x.md\npackage x\n")
                (root / "ai" / "DOCS-TO-CODE.md").write_text("v2\n")
                problems = ch.discovery_index_problems(root, ("internal/x/y.go",))
                self.assertTrue(problems, "a fed-but-omitted dirty index must refuse")
                self.assertIn("DOCS-TO-CODE.md", "\n".join(problems))
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


class TestBrokenHeadFixEscape(unittest.TestCase):
    """A broken HEAD must not block the commit that fixes it.

    VALIDATES: `ze-tracked-build-check` is the one structural gate whose red
    lives in HEAD, not in the working tree, so it is cleared BY a commit rather
    than before one. `--broken-head-fix "<reason>"` lets that commit through, and
    only that one: the escape applies when tracked-build is the ONLY structural
    red.
    PREVENTS: the deadlock the gate would otherwise create. A commit that lands a
    consumer without its producer reddens tracked-build; the structural refusal
    then blocks every commit including the one landing the producer, leaving the
    owner-only `--structural-red-ok` as the sole route. HEAD stays broken for
    everybody who builds it until the owner is available.
    """

    def _run(self, reds: list[str], extra: list[str]):
        import contextlib
        import io

        saved = (ch.verify_status, ch.structural_gate_reds)
        ch.verify_status = lambda repo: ("stale", "structural red")
        ch.structural_gate_reds = lambda repo: reds
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
                        ]
                        + extra
                    )
                return rc, err.getvalue() + out.getvalue()
        finally:
            ch.verify_status, ch.structural_gate_reds = saved

    def test_a_broken_head_alone_blocks_without_the_flag(self):
        rc, msg = self._run([ch.TRACKED_BUILD_GATE], [])
        self.assertEqual(rc, 2, msg)
        self.assertIn("STRUCTURAL GATE", msg)

    def test_the_refusal_names_the_escape(self):
        # Telling the reader to "re-run a full verify until green" is advice that
        # cannot work here: only a commit clears this red.
        _, msg = self._run([ch.TRACKED_BUILD_GATE], [])
        self.assertIn("--broken-head-fix", msg)

    def test_the_flag_lets_the_fixing_commit_through(self):
        rc, msg = self._run(
            [ch.TRACKED_BUILD_GATE],
            [
                "--broken-head-fix",
                "lands the PeerInfo field peer.go already reads",
                "--unverified",
                "record is stale because HEAD does not compile",
            ],
        )
        self.assertEqual(rc, 0, msg)
        # Loud, or a broken HEAD reads the same as a green one in the transcript.
        self.assertIn("HEAD does not compile", msg)

    def test_the_flag_requires_a_reason(self):
        rc, msg = self._run([ch.TRACKED_BUILD_GATE], ["--broken-head-fix", "   "])
        self.assertEqual(rc, 2, msg)
        # Assert the STRUCTURAL refusal specifically: a blank reason would exit 2
        # through the later not-FRESH-green check too, so the exit code alone
        # does not discriminate.
        self.assertIn("STRUCTURAL GATE", msg)

    def test_it_does_not_double_as_an_unverified(self):
        # It clears the STRUCTURAL refusal and nothing else. `verify_status` goes
        # stale for flaky test reds and for age too, and a reason written about
        # HEAD does not speak for those, so --unverified is still required.
        rc, msg = self._run([], ["--broken-head-fix", "no structural red here"])
        self.assertEqual(rc, 2, msg)
        self.assertIn("FRESH-green", msg)

    def test_it_does_not_wave_through_a_stale_record_even_when_head_is_broken(self):
        # The half the previous case misses: tracked-build IS red, the flag IS
        # applied, and the record may ALSO be stale for reasons the reason given
        # says nothing about. --unverified is still owed.
        rc, msg = self._run(
            [ch.TRACKED_BUILD_GATE], ["--broken-head-fix", "lands the missing producer"]
        )
        self.assertEqual(rc, 2, msg)
        self.assertIn("FRESH-green", msg)

    def test_the_refusal_names_both_flags(self):
        _, msg = self._run([ch.TRACKED_BUILD_GATE], [])
        self.assertIn("--broken-head-fix", msg)
        self.assertIn("--unverified", msg)

    def test_it_cannot_wave_through_another_structural_red(self):
        # The narrowness IS the guard: a lint or tier red riding alongside a
        # broken HEAD must still refuse.
        rc, msg = self._run(
            [ch.TRACKED_BUILD_GATE, "ze-tier-check"],
            ["--broken-head-fix", "lands the missing producer"],
        )
        self.assertEqual(rc, 2, msg)
        self.assertIn("ze-tier-check", msg)


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
                + "| 2026-07-21 | spec-a | work A | reason | `plan/spec-a.md` | deferred |\n"  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
            )
            (root / b).write_text(
                DEFERRALS_HEADER
                + "| 2026-07-21 | spec-b | work B | reason | `plan/spec-b.md` | deferred |\n"  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
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
                + "| 2026-07-21 | spec-a | work A | reason | `plan/spec-a.md` | deferred |\n"  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
                + "| 2026-07-21 | spec-b | work B | reason | `plan/spec-b.md` | deferred |\n"  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
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
                + "| 2026-07-21 | spec-real | ok | reason | `plan/spec-real.md` | deferred |\n"  # <!-- doc-links: ignore (fixture path, created in a temporary repository) -->
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
            wiring="| config | `test/parse/a.ci` | read, covers path |\n",  # <!-- doc-links: ignore (fixture path in a wiring row, not a real test) -->
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


class TestJournalRowIsAClosureSignal(unittest.TestCase):
    """A journal row names the spec a commit closes.

    `spec_closure_stem` reads the Spec cell of the row this commit ADDS, and
    `closure_reminder` nudges for the second closure commit. Both serve the
    two-commit spec closure in `ai/rules/planning.md`.
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

    # VALIDATES: AC-7 -- spec_closure_stem() reads the Spec cell from a journal
    # row and returns it as the closure stem, so review_gate_problems() fires on
    # the commit that carries the code.
    def test_journal_row_is_a_closure_signal(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            journal_dir = root / "plan" / "journal"
            journal_dir.mkdir(parents=True)
            (journal_dir / "some-class.md").write_text(
                "| Date | Spec | Surface | Symptom | Fix |\n"
                "|------|------|---------|---------|-----|\n"
                "| 2026-08-09 | my-feature | gate | it refused | fixed |\n"
            )
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertEqual(stem, "my-feature")

    # VALIDATES: spec_closure_stem() returns None for a journal row whose Spec
    # cell is "-", which marks a row written outside a spec.
    def test_journal_dash_spec_is_not_a_closure_signal(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            journal_dir = root / "plan" / "journal"
            journal_dir.mkdir(parents=True)
            (journal_dir / "some-class.md").write_text(
                "| Date | Spec | Surface | Symptom | Fix |\n"
                "|------|------|---------|---------|-----|\n"
                "| 2026-08-09 | - | gate | it refused | fixed |\n"
            )
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertIsNone(stem)

    _JOURNAL_HEAD = (
        "| Date | Spec | Surface | Symptom | Fix |\n"
        "|------|------|---------|---------|-----|\n"
        "| 2026-07-01 | old-spec | gate | first time | fixed |\n"
    )

    def _journal_repo(self, tmp: str) -> tuple[Path, Path]:
        """A repo whose HEAD already holds a one-row journal class file."""
        root = self._repo(tmp)
        journal = root / "plan" / "journal"
        journal.mkdir(parents=True)
        path = journal / "some-class.md"
        path.write_text(self._JOURNAL_HEAD)
        (root / "plan" / "spec-other.md").write_text("# Spec: other\n")
        _git(root, "add", "-A")
        _git(root, "commit", "-q", "-m", "first closure through this class")
        return root, path

    def _claim(self, root: Path, spec: str) -> None:
        """Make `claimed_spec()` report `spec` for this fixture repo.

        The real marker lives outside the repo, so the fixture supplies the one
        thing `claimed_spec` reads: `scripts/dev/spec-session.sh current`.
        """
        script = root / "scripts" / "dev" / "spec-session.sh"
        script.write_text(f'#!/bin/sh\n[ "$1" = current ] && echo {spec}\n')
        script.chmod(0o755)

    # VALIDATES: the closure stem comes from the row this commit ADDS, not from
    # the first row in the file. A class file is multi-row by design.
    # PREVENTS: from the second closure through a class onward,
    # review_gate_problems() being handed a spec that closed long ago, so the
    # review gate blocks or passes on the wrong spec.
    def test_journal_stem_comes_from_the_added_row(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(
                self._JOURNAL_HEAD
                + "| 2026-08-09 | new-spec | gate | second time | fixed |\n"
            )
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertEqual(stem, "new-spec")

    # VALIDATES: a row already at HEAD is not a closure signal, the way a learned
    # summary already at HEAD is not one (_tracked_at_head).
    # PREVENTS: every later commit that touches the class file being read as a
    # re-closure of the spec its first row names.
    def test_journal_row_already_at_head_is_not_a_closure_signal(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(self._JOURNAL_HEAD + "\nSome prose, no new row.\n")
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertIsNone(stem)

    # VALIDATES: a re-formatted row is not an added row. Column-padding a class
    # file re-emits every row as `+`, and the first of them names whatever spec
    # closed through that class FIRST.
    # PREVENTS: the stale-stem failure arriving by a second route. The shards are
    # unpadded today, so this is one `column -t` away rather than hypothetical.
    # The `+` and the `-` carry the same cells once stripped, so matching them
    # leaves only the row whose CONTENT is new.
    def test_a_repadded_row_is_not_a_new_row(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(
                "| Date       | Spec     | Surface | Symptom    | Fix   |\n"
                "|------------|----------|---------|------------|-------|\n"
                "| 2026-07-01 | old-spec | gate    | first time | fixed |\n"
            )
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertIsNone(stem, "a re-pad adds no occurrence")

    # VALIDATES: the control for the case above -- a re-pad that ALSO appends a
    # genuinely new row still yields that row's spec, and never the padded one.
    def test_a_repad_plus_a_new_row_yields_the_new_row(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(
                "| Date       | Spec     | Surface | Symptom     | Fix   |\n"
                "|------------|----------|---------|-------------|-------|\n"
                "| 2026-07-01 | old-spec | gate    | first time  | fixed |\n"
                "| 2026-08-09 | new-spec | gate    | second time | fixed |\n"
            )
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertEqual(stem, "new-spec")

    # An edit to an older row plus the closure row: two surviving `+` lines whose
    # cells match no `-`, the edited one first because diff order is file order.
    _EDIT_PLUS_APPEND = (
        "| Date | Spec | Surface | Symptom | Fix |\n"
        "|------|------|---------|---------|-----|\n"
        "| 2026-07-01 | old-spec | gate | first occurrence | fixed |\n"
        "| 2026-08-09 | new-spec | gate | second time | fixed |\n"
    )

    # VALIDATES: an EDIT to an older row is new content (its cells match no `-`),
    # so the reader returns it AND the closure row, and the caller chooses.
    # PREVENTS: the shape the cell comparison does not cancel. Fixing a typo in a
    # months-old Symptom cell while appending the closure row made the FIRST
    # survivor the old row, and the single-answer reader returned a months-old
    # stem to both gates that read it.
    def test_an_edited_older_row_does_not_hide_the_closure_row(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(self._EDIT_PLUS_APPEND)
            stems = ch._journal_added_spec_stems(
                root, ("plan/journal/some-class.md",), ()
            )
            self.assertEqual(stems, ["old-spec", "new-spec"])

    # VALIDATES: with two stems the session's CLAIM decides which one this commit
    # closes, so the review artifact is keyed on the spec being closed.
    # PREVENTS: review_gate.py being asked for `old-spec`'s artifact while
    # `new-spec` is the spec closing, which no session can satisfy.
    def test_the_claimed_spec_decides_between_two_added_stems(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(self._EDIT_PLUS_APPEND)
            self._claim(root, "spec-new-spec.md")
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertEqual(stem, "new-spec")

    # VALIDATES: no claim leaves the first stem answering, so the gate still
    # fires on a closure prepared by a session that claimed nothing.
    # PREVENTS: the claim becoming an override -- returning None with no claim
    # would drop the review gate off the commit carrying the code.
    # NOT EVIDENCE FOR the claim tie-break beside it: this case passes with that
    # tie-break reverted, because the fallback answers the same way. It is a
    # regression guard over the fallback, and it is meant to keep passing.
    def test_no_claim_still_yields_a_stem(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(self._EDIT_PLUS_APPEND)
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertEqual(stem, "old-spec")

    # An edit to the ONE older row, appending nothing: a code-free journal typo
    # fix. One surviving `+` line, so one stem, so the claim tie-break never runs.
    _EDIT_ONLY = (
        "| Date | Spec | Surface | Symptom | Fix |\n"
        "|------|------|---------|---------|-----|\n"
        "| 2026-07-01 | old-spec | gate | first occurrence | fixed |\n"
    )

    def _close_spec(self, root: Path, stem: str) -> None:
        """Close plan/spec-<stem>.md the way ai/rules/planning.md does.

        Two commits: the spec exists, then commit B `git rm`s it. That leaves the
        state every spec that closed EARLIER is in, which is what
        `_spec_closed_earlier` reads.
        """
        rel = f"plan/spec-{stem}.md"
        (root / rel).write_text(f"# Spec: {stem}\n")
        _git(root, "add", "-A")
        _git(root, "commit", "-q", "-m", f"spec {stem}")
        _git(root, "rm", "-q", rel)
        _git(root, "commit", "-q", "-m", f"close {stem}")

    # VALIDATES: a commit that only EDITS an older row closes nothing, because the
    # spec that row names has no plan/spec-<stem>.md left -- commit B removed it.
    # PREVENTS: the single-stem shape of the stale-stem failure. One stem never
    # reaches the claim tie-break, so the months-old stem answered and
    # review_gate_problems() demanded a clean artifact for a spec nobody was
    # closing, refusing an ordinary code-free journal typo fix.
    def test_an_edit_only_commit_on_a_closed_spec_is_not_a_closure(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            self._close_spec(root, "old-spec")
            path.write_text(self._EDIT_ONLY)
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertIsNone(stem, "a closed spec cannot be closing again")

    # VALIDATES: the control -- the same edit-only shape on a spec still OPEN on
    # disk still names it, so the gate keeps firing where it should.
    # PREVENTS: the drop widening into "a journal row never closes anything",
    # which would take the review gate off every journal-borne closure.
    def test_an_edit_only_commit_on_an_open_spec_still_yields_its_stem(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            (root / "plan" / "spec-old-spec.md").write_text("# Spec: old-spec\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-q", "-m", "spec old-spec is open")
            path.write_text(self._EDIT_ONLY)
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertEqual(stem, "old-spec")

    # VALIDATES: a Spec cell naming a path git has NEVER held is kept, not
    # dropped. Absent-from-disk alone does not mean closed.
    # PREVENTS: a misspelled Spec cell on a real closure commit disarming the
    # review gate silently -- the fail-OPEN the "gone, having once existed" test
    # exists to refuse.
    def test_a_spec_git_never_held_still_yields_its_stem(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(
                self._JOURNAL_HEAD
                + "| 2026-08-09 | mispelt-spec | gate | second time | fixed |\n"
            )
            stem = ch.spec_closure_stem(("plan/journal/some-class.md",), (), root)
            self.assertEqual(stem, "mispelt-spec")

    # VALIDATES: `closure_reminder` applies the same closed-spec filter as
    # `spec_closure_stem`, so an edit-only commit on a spec that closed earlier
    # is not nudged to prepare a commit B for it.
    # PREVENTS: the two call sites of `_journal_added_spec_stems` disagreeing
    # about what a closure is. The filter landed in one and not the other, and
    # nothing failed, because this function had no test at all.
    def test_closure_reminder_is_silent_on_an_edit_to_a_closed_specs_row(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            self._close_spec(root, "old-spec")
            path.write_text(self._EDIT_ONLY)
            note = ch.closure_reminder(("plan/journal/some-class.md",), (), root)
            self.assertIsNone(note, "a spec that closed earlier needs no commit B")

    # VALIDATES: the control -- a real closure still gets its nudge, so the
    # filter narrowed the false positive without taking the reminder away.
    # PREVENTS: the filter widening into "a journal row never nudges", which
    # would drop the guard against the orphaned in-progress spec the two-commit
    # closure exists to avoid.
    def test_closure_reminder_still_fires_for_a_spec_open_on_disk(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            (root / "plan" / "spec-old-spec.md").write_text("# Spec: old-spec\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-q", "-m", "spec old-spec is open")
            path.write_text(self._EDIT_ONLY)
            note = ch.closure_reminder(("plan/journal/some-class.md",), (), root)
            self.assertIsNotNone(note)
            self.assertIn("closure-reminder", note or "")

    # VALIDATES: the Pre-Commit Verification gate fires on the closure row even
    # when an older row is edited in the same commit.
    # PREVENTS: `stem in _journal_added_spec_stems(...)` reading False because
    # the reader answered with the edited row, which let a closure commit land
    # with the spec's evidence tables byte-identical to the template.
    def test_spec_audit_fires_on_the_closure_row_beside_an_edit(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(self._EDIT_PLUS_APPEND)
            (root / "plan" / "spec-new-spec.md").write_text(
                _pcv(files="| `a.go` | yes | ls output |\n")
            )
            problems = ch.spec_audit_problems(
                root, ("plan/journal/some-class.md",), "spec-new-spec.md", ()
            )
            self.assertTrue(problems, "the closure row must reach the audit gate")
            self.assertIn("AC Verified", problems[0])

    # VALIDATES: commit B derives the stem from the spec it REMOVES, even when it
    # also carries a journal file.
    # PREVENTS: the journal loop answering first and handing commit B the stem of
    # a spec that closed through the same class earlier.
    def test_removed_spec_beats_a_journal_row(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, path = self._journal_repo(tmp)
            path.write_text(
                self._JOURNAL_HEAD
                + "| 2026-08-09 | new-spec | gate | second time | fixed |\n"
            )
            stem = ch.spec_closure_stem(
                ("plan/journal/some-class.md",), ("plan/spec-other.md",), root
            )
            self.assertEqual(stem, "other")


class TestScriptPathIsUniquePerPreparedCommit(unittest.TestCase):
    """A prepared commit owns its own script path.

    VALIDATES: `ai/rules/git-safety.md` "Commit Rules" -- the `script=` line is
    the authoritative path and callers never rebuild it. Keying the script on the
    Claude session was enough while a session was one agent. One session now runs
    many subagents, they all resolve to one fingerprint, and they shared
    `tmp/commit-<SESSION>.sh`: measured 2026-08-05, one session produced 53
    message files against 18 scripts, and a `--replace` from one agent left
    another's 20-file commit reachable only through its surviving message file.
    PREVENTS: a second `create` overwriting the first's prepared script, and a
    caller reaching a foreign script by reconstructing the path convention.
    """

    def _repo(self, tmp: str) -> Path:
        root = Path(tmp)
        _git(root, "init", "-q")
        _git(root, "config", "user.email", "t@example.com")
        _git(root, "config", "user.name", "t")
        _git(root, "config", "commit.gpgsign", "false")
        for name in ("one.txt", "two.txt"):
            (root / name).write_text(name + "\n")
        _git(root, "add", "-A")
        _git(root, "commit", "-qm", "init")
        return root

    def _create(self, root: Path, *extra: str):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = ch.main(
                [
                    "--repo",
                    str(root),
                    "create",
                    "--session",
                    "abcd1234",
                    *extra,
                ]
            )
        return rc, out.getvalue(), err.getvalue()

    def _script_line(self, stdout: str) -> str:
        for line in stdout.splitlines():
            if line.startswith("script="):
                return line[len("script=") :]
        self.fail(f"no script= line in:\n{stdout}")

    def test_two_creates_get_two_scripts_and_neither_is_destroyed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out_a, err = self._create(
                root, "--subject", "first", "--file", "one.txt"
            )
            self.assertEqual(rc, 0, err)
            first = self._script_line(out_a)
            rc, out_b, err = self._create(
                root, "--subject", "second", "--file", "two.txt"
            )
            self.assertEqual(rc, 0, err)
            second = self._script_line(out_b)

            self.assertNotEqual(first, second)
            # Both survive, and each still commits ITS OWN file set.
            text_a = (root / first).read_text()
            text_b = (root / second).read_text()
            self.assertIn("one.txt", text_a)
            self.assertNotIn("two.txt", text_a)
            self.assertIn("two.txt", text_b)
            self.assertNotIn("one.txt", text_b)
            # And the message file each names is the one it was written with.
            self.assertIn("git commit -F tmp/commit-msg-abcd1234-a.txt", text_a)
            self.assertIn("git commit -F tmp/commit-msg-abcd1234-b.txt", text_b)

    def test_the_path_is_not_reconstructible_by_convention(self):
        """The name carries a random component, so a guess cannot hit it.

        Guessing is what bit the session this change came from: a path copied on
        the belief it was one's own belonged to another agent.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root, "--subject", "first", "--tag", "a", "--file", "one.txt"
            )
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            self.assertNotIn(
                script, ("tmp/commit-abcd1234.sh", "tmp/commit-abcd1234-a.sh")
            )
            self.assertRegex(script, r"^tmp/commit-abcd1234-a-[0-9a-f]{6}\.sh$")

    def test_append_still_adds_a_second_block_to_the_named_script(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(root, "--subject", "first", "--file", "one.txt")
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            rc, out2, err = self._create(
                root,
                "--subject",
                "second",
                "--file",
                "two.txt",
                "--append",
                "--script",
                script,
            )
            self.assertEqual(rc, 0, err)
            self.assertEqual(self._script_line(out2), script)
            text = (root / script).read_text()
            self.assertIn("git commit -F tmp/commit-msg-abcd1234-a.txt", text)
            self.assertIn("git commit -F tmp/commit-msg-abcd1234-b.txt", text)
            self.assertEqual(text.count("#!/bin/bash"), 1)

    def test_append_without_script_resolves_only_when_unambiguous(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(root, "--subject", "first", "--file", "one.txt")
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            # One script exists: the legacy bare --append keeps working.
            rc, out2, err = self._create(
                root, "--subject", "second", "--file", "two.txt", "--append"
            )
            self.assertEqual(rc, 0, err)
            self.assertEqual(self._script_line(out2), script)
            # A sibling agent prepares its own: now the guess is refused, named.
            rc, out3, err = self._create(
                root, "--subject", "third", "--file", "one.txt"
            )
            self.assertEqual(rc, 0, err)
            other = self._script_line(out3)
            rc, _, err = self._create(
                root, "--subject", "fourth", "--file", "two.txt", "--append"
            )
            self.assertEqual(rc, 2, err)
            self.assertIn("--append is ambiguous", err)
            self.assertIn(script, err)
            self.assertIn(other, err)

    def test_replace_refuses_a_script_prepared_for_another_commit(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root, "--subject", "theirs", "--file", "two.txt"
            )
            self.assertEqual(rc, 0, err)
            theirs = self._script_line(out)
            before = (root / theirs).read_text()
            rc, _, err = self._create(
                root,
                "--subject",
                "mine",
                "--file",
                "one.txt",
                "--replace",
                "--script",
                theirs,
            )
            self.assertEqual(rc, 2, err)
            self.assertIn("--replace refused", err)
            self.assertIn("two.txt", err)
            self.assertEqual((root / theirs).read_text(), before)

    def test_replace_still_amends_my_own_script(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(root, "--subject", "mine", "--file", "one.txt")
            self.assertEqual(rc, 0, err)
            mine = self._script_line(out)
            rc, out2, err = self._create(
                root,
                "--subject",
                "mine, with the file I forgot",
                "--file",
                "one.txt",
                "--file",
                "two.txt",
                "--replace",
                "--script",
                mine,
            )
            self.assertEqual(rc, 0, err)
            self.assertEqual(self._script_line(out2), mine)
            text = (root / mine).read_text()
            self.assertIn("two.txt", text)
            self.assertEqual(text.count("#!/bin/bash"), 1)

    def test_the_staging_guard_names_the_owning_script(self):
        """The guard that saved the 20-file commit must say which script aborted.

        Not weakened: the grep and the abort are unchanged, one line is added.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(root, "--subject", "first", "--file", "one.txt")
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            text = (root / script).read_text()
            self.assertIn("ABORT: index has staged files not in this commit", text)
            self.assertIn("this script: " + script, text)

    def test_tags_are_reserved_atomically(self):
        """Two allocations inside one `create` window must not agree.

        The message file is written at the END of `create`, seconds after the tag
        is chosen, so an allocator that only globs the tags on disk hands the
        same letter to both agents and the second message file overwrites the
        first. That message file was the only copy of a 20-file commit.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "tmp").mkdir(exist_ok=True)
            first = ch.next_tag(root, "abcd1234")
            second = ch.next_tag(root, "abcd1234")
            self.assertNotEqual(first, second)
            self.assertTrue((root / ch.message_rel_path("abcd1234", first)).exists())

    def test_a_refused_create_gives_its_tag_back(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, _, err = self._create(root, "--subject", "bad", "--file", "absent.txt")
            self.assertEqual(rc, 2, err)
            self.assertEqual(list((root / "tmp").glob("commit-msg-*.txt")), [])
            rc, out, err = self._create(root, "--subject", "good", "--file", "one.txt")
            self.assertEqual(rc, 0, err)
            self.assertIn("message=tmp/commit-msg-abcd1234-a.txt", out)


class _ScriptFixture:
    """A repo, a `create` driver, and a git shim that records argv.

    Shared by the push tests and the caller-text-injection tests: both ask what a
    GENERATED SCRIPT does when bash runs it, and neither may make a network call.
    """

    def _repo(self, tmp: str) -> Path:
        root = Path(tmp)
        _git(root, "init", "-q")
        _git(root, "config", "user.email", "t@example.com")
        _git(root, "config", "user.name", "t")
        _git(root, "config", "commit.gpgsign", "false")
        for name in ("one.txt", "two.txt"):
            (root / name).write_text(name + "\n")
        _git(root, "add", "-A")
        _git(root, "commit", "-qm", "init")
        (root / "one.txt").write_text("one changed\n")
        (root / "two.txt").write_text("two changed\n")
        return root

    def _run_create(self, root: Path, *extra: str):
        """Drive `create` with exactly the flags given, capturing both streams."""
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = ch.main(
                ["--repo", str(root), "create", "--session", "abcd1234", *extra]
            )
        return rc, out.getvalue(), err.getvalue()

    def _create(self, root: Path, *extra: str):
        return self._run_create(root, *extra)

    def _script_line(self, stdout: str) -> str:
        for line in stdout.splitlines():
            if line.startswith("script="):
                return line[len("script=") :]
        self.fail(f"no script= line in:\n{stdout}")

    PUSH_LINE = "git " + "push"

    def _push_lines(self, text: str) -> list[int]:
        return [
            index
            for index, line in enumerate(text.splitlines())
            if line.strip() == self.PUSH_LINE
        ]

    def _run_script(self, root: Path, script: str) -> tuple[int, list[str]]:
        """Run a generated script with a git shim, return (exit code, git argv log).

        The shim records every git invocation and refuses to make a real network
        call: the question these tests ask is WHETHER the push was reached, and a
        recorded argv answers it without a remote.
        """
        shim_dir = root / "shim"
        shim_dir.mkdir(exist_ok=True)
        log = root / "git-calls.log"
        real_git = shutil.which("git")
        self.assertIsNotNone(real_git, "git is required for this test")
        shim = shim_dir / "git"
        shim.write_text(
            "#!/bin/bash\n"
            'printf "%s\\n" "$*" >> "$GIT_CALL_LOG"\n'
            'if [ "$1" = "' + self.PUSH_LINE.split()[1] + '" ]; then exit 0; fi\n'
            "exec " + real_git + ' "$@"\n'
        )
        shim.chmod(0o755)
        env = dict(os.environ)
        env["PATH"] = str(shim_dir) + os.pathsep + env["PATH"]
        env["GIT_CALL_LOG"] = str(log)
        proc = subprocess.run(
            ["bash", str(root / script)],
            cwd=str(root),
            env=env,
            capture_output=True,
            text=True,
        )
        calls = log.read_text().splitlines() if log.exists() else []
        return proc.returncode, calls


class TestPushIsAnOwnerInstruction(_ScriptFixture, unittest.TestCase):
    """`--push` publishes a prepared commit, and only on the owner's order.

    VALIDATES: `ai/rules/git-safety.md` -- the ban on a bare push exists because
    concurrent sessions share one index, and the generated script is what made
    staging atomic. A push INSIDE that script inherits the same atomicity: it is
    reached only when every commit block above it succeeded.
    PREVENTS: a push that fires after a failed commit (publishing a tree nobody
    prepared), a push nobody authorised, and an --append that lands a commit
    BELOW the push meant to publish it.
    """

    def test_without_the_flag_no_push_reaches_the_script(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(root, "--subject", "first", "--file", "one.txt")
            self.assertEqual(rc, 0, err)
            text = (root / self._script_line(out)).read_text()
            self.assertEqual(self._push_lines(text), [])
            self.assertNotIn(ch.PUSH_MARKER, text)
            self.assertNotIn("push=AUTHORISED", out)

    def test_the_flag_puts_exactly_one_push_at_the_very_end(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered the push, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            text = (root / self._script_line(out)).read_text()
            lines = [line for line in text.splitlines() if line.strip()]
            self.assertEqual(len(self._push_lines(text)), 1)
            self.assertEqual(lines[-1].strip(), self.PUSH_LINE)
            # ... and after the commit it publishes, never before it.
            body = text.splitlines()
            commit_at = max(
                i for i, line in enumerate(body) if line.startswith("git commit -F ")
            )
            self.assertGreater(self._push_lines(text)[0], commit_at)

    def test_the_authorisation_is_recorded_in_the_script_and_on_stdout(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered the push, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            text = (root / self._script_line(out)).read_text()
            self.assertIn(ch.PUSH_MARKER + " Thomas ordered the push, 2026-08-05", text)
            self.assertIn("push=AUTHORISED (Thomas ordered the push, 2026-08-05)", out)

    def test_a_multiline_authorisation_cannot_escape_its_comment(self):
        """A newline in the reason would end the comment and run what follows."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered it\nrm -rf /tmp/nothing-here",
            )
            self.assertEqual(rc, 0, err)
            text = (root / self._script_line(out)).read_text()
            self.assertNotIn("\nrm -rf", text)
            self.assertIn(
                ch.PUSH_MARKER + " Thomas ordered it rm -rf /tmp/nothing-here", text
            )

    def test_an_authorisation_too_short_to_name_anyone_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, _, err = self._create(
                root, "--subject", "first", "--file", "one.txt", "--push", "ok"
            )
            self.assertEqual(rc, 2, err)
            self.assertIn("--push authorisation is too short", err)
            self.assertEqual(list((root / "tmp").glob("commit-*.sh")), [])

    def test_append_after_a_push_keeps_it_last_and_single(self):
        """The sharp edge: block two must land INSIDE the push, not below it."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered the push, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            rc, out2, err = self._create(
                root,
                "--subject",
                "second",
                "--file",
                "two.txt",
                "--append",
                "--script",
                script,
            )
            self.assertEqual(rc, 0, err)
            text = (root / script).read_text()
            self.assertEqual(len(self._push_lines(text)), 1)
            self.assertEqual(text.count(ch.PUSH_MARKER), 1)
            commits = [
                index
                for index, line in enumerate(text.splitlines())
                if line.startswith("git commit -F ")
            ]
            self.assertEqual(len(commits), 2)
            self.assertGreater(self._push_lines(text)[0], commits[-1])
            # The authorisation the owner gave survives the append, and the
            # caller is told the script they now hold will publish both commits.
            self.assertIn("push=AUTHORISED (Thomas ordered the push, 2026-08-05)", out2)

    def test_a_repeated_push_on_the_append_re_authorises_it_once(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered the push, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            rc, _, err = self._create(
                root,
                "--subject",
                "second",
                "--file",
                "two.txt",
                "--append",
                "--script",
                script,
                "--push",
                "Thomas re-ordered it for both commits, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            text = (root / script).read_text()
            self.assertEqual(text.count(ch.PUSH_MARKER), 1)
            self.assertEqual(len(self._push_lines(text)), 1)
            self.assertIn("Thomas re-ordered it for both commits", text)

    def test_the_push_runs_only_after_every_commit_succeeded(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered the push, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            code, calls = self._run_script(root, self._script_line(out))
            self.assertEqual(code, 0, calls)
            pushes = [i for i, call in enumerate(calls) if call.startswith("push")]
            commits = [i for i, call in enumerate(calls) if call.startswith("commit ")]
            self.assertEqual(len(pushes), 1, calls)
            self.assertTrue(commits, calls)
            self.assertGreater(pushes[0], commits[-1], calls)

    def test_the_push_never_runs_when_a_commit_fails(self):
        """A push after a failed commit publishes a tree nobody prepared."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered the push, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            # Break the commit the way a real one breaks: the message file the
            # block names is gone, so `git commit -F` exits nonzero.
            (root / "tmp" / "commit-msg-abcd1234-a.txt").unlink()
            code, calls = self._run_script(root, script)
            self.assertNotEqual(code, 0, calls)
            self.assertEqual([c for c in calls if c.startswith("push")], [], calls)


class TestCallerTextCannotBecomeScript(_ScriptFixture, unittest.TestCase):
    """No caller string may reach the generated script as a command.

    VALIDATES: `ai/rules/git-safety.md` -- the generated script is the ONE place a
    commit is allowed to happen, so what it contains has to come from this helper,
    not from a value a caller passed.
    PREVENTS: the forgery an adversarial review found -- a line that spells
    `# ze-commit-push:` anywhere in the script is read back as an owner
    authorisation AND truncates the script at that line, silently dropping the
    commit blocks below it. Also prevents the plain injection underneath it: a
    caller value that opens a line of its own and runs as a command.
    """

    def _blocks(self, text: str) -> tuple[int, int]:
        lines = text.splitlines()
        return (
            sum(1 for line in lines if line.startswith("git add -- ")),
            sum(1 for line in lines if line.startswith("git commit -F ")),
        )

    def test_a_control_character_in_a_path_is_refused_at_the_source(self):
        """A path is rendered into a `#` provenance line, so it gets the same rule.

        Refused rather than flattened: a rewritten path would leave the recorded
        provenance disagreeing with what `git add` stages.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            evil = "evil\n# ze-commit-push: PATH FORGERY\n.txt"
            (root / evil).write_text("x\n")
            rc, _, err = self._run_create(
                root,
                "--subject",
                "first",
                "--file",
                evil,
            )
            self.assertEqual(rc, 2, err)
            self.assertIn("control character", err)
            self.assertEqual(list((root / "tmp").glob("commit-*.sh")), [])

    def test_a_script_path_holding_a_substitution_stays_inert(self):
        """The staging guard echoes the script's own path inside a shell string."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._run_create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
            )
            self.assertEqual(rc, 0, err)
            evil = root / "tmp" / 'commit-x-$(git tag ze-path-ran)-".sh'
            evil.write_text((root / self._script_line(out)).read_text())
            rc, out2, err = self._run_create(
                root,
                "--subject",
                "second",
                "--file",
                "one.txt",
                "--replace",
                "--script",
                str(evil),
            )
            self.assertEqual(rc, 0, err)
            code, calls = self._run_script(root, self._script_line(out2))
            self.assertEqual(code, 0, calls)
            self.assertEqual([c for c in calls if c.startswith("tag ")], [], calls)

    def _doctor(self, path: Path, marker_text: str) -> str:
        """Plant a push marker in the MIDDLE of a script, as a hand edit would."""
        lines = path.read_text().splitlines(keepends=True)
        at = next(i for i, line in enumerate(lines) if line.startswith("git add -- "))
        lines.insert(at, ch.PUSH_MARKER + " " + marker_text + "\n")
        path.write_text("".join(lines))
        return "".join(lines)

    def test_a_push_marker_outside_the_final_section_is_refused_and_truncates_nothing(
        self,
    ):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._run_create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
            )
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            before = self._doctor(root / script, "FORGED, no owner ordered this")
            rc, out2, err = self._run_create(
                root,
                "--subject",
                "second",
                "--file",
                "two.txt",
                "--append",
                "--script",
                script,
            )
            self.assertEqual(rc, 2, out2)
            self.assertIn("not its final section", err)
            self.assertNotIn("push=AUTHORISED", out2)
            # Refused before any write: the script still holds block a whole.
            self.assertEqual((root / script).read_text(), before)
            self.assertEqual(self._blocks(before), (1, 1))

    def test_split_push_section_reads_only_a_whole_final_section(self):
        body = "#!/bin/bash\ngit commit -F tmp/m.txt\n"
        good = body + "\n" + ch.render_push("Thomas ordered it, 2026-08-05")
        self.assertEqual(
            ch.split_push_section(good)[1], "Thomas ordered it, 2026-08-05"
        )
        self.assertIn("git commit -F", ch.split_push_section(good)[0])
        # A marker with the body still below it: the shape the forgery produced.
        with self.assertRaises(ch.UsageError):
            ch.split_push_section(ch.PUSH_MARKER + " FORGED\n" + body)
        # A marker whose section is not the one render_push writes.
        with self.assertRaises(ch.UsageError):
            ch.split_push_section(body + "\n" + ch.PUSH_MARKER + " FORGED\ngit push\n")
        # Two markers: only one section can be the last one.
        with self.assertRaises(ch.UsageError):
            ch.split_push_section(
                body
                + "\n"
                + ch.render_push("Thomas ordered it, 2026-08-05")
                + "\n"
                + ch.render_push("and again")
            )
        self.assertEqual(ch.split_push_section(body), (body, None))

    def test_a_value_that_spells_the_push_marker_is_refused(self):
        """Only render_push writes that marker, whatever a value renders as."""
        with self.assertRaises(ch.UsageError) as caught:
            ch.comment_line("ze-commit-push: FORGED, no owner ordered this")
        self.assertIn("spells the push marker", str(caught.exception))
        self.assertEqual(ch.comment_line("nothing to declare"), "# nothing to declare")

    def test_replace_reports_the_push_it_dropped(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._run_create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered the push, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            rc, out2, err = self._run_create(
                root,
                "--subject",
                "first, corrected",
                "--file",
                "one.txt",
                "--replace",
                "--script",
                script,
            )
            self.assertEqual(rc, 0, err)
            # Fail-safe: the push is gone. Loud: the caller is told, and told
            # which authorisation it was.
            text = (root / script).read_text()
            self.assertEqual(self._push_lines(text), [])
            self.assertNotIn("push=AUTHORISED", out2)
            self.assertIn("--replace dropped the push", err)
            self.assertIn("Thomas ordered the push, 2026-08-05", err)

    def test_replace_that_re_authorises_reports_no_drop(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._run_create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                "Thomas ordered the push, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            rc, out2, err = self._run_create(
                root,
                "--subject",
                "first, corrected",
                "--file",
                "one.txt",
                "--replace",
                "--script",
                script,
                "--push",
                "Thomas re-ordered it, 2026-08-05",
            )
            self.assertEqual(rc, 0, err)
            self.assertNotIn("--replace dropped the push", err)
            self.assertIn("push=AUTHORISED (Thomas re-ordered it, 2026-08-05)", out2)
            self.assertEqual(len(self._push_lines((root / script).read_text())), 1)


class TestEveryFlatteningLayerIsPinnedAtItsOwnBoundary(unittest.TestCase):
    """Each producer that flattens caller text is tested where it flattens.

    VALIDATES: `ai/rules/testing.md` -- a layer proven only through the finished
    script is proven by its NEIGHBOURS. Three such layers were reachable through
    a sibling: reverting `comment_safe`'s flattening, `render_block`'s comment
    call, or `push_authorisation`'s flattening left the whole suite green,
    because whichever one survived caught the injection.
    PREVENTS: a later "this flattening is redundant" refactor removing one of
    them and staying green until an input arrives that the surviving layer does
    not see.
    """

    # Two payloads in one string, because the two ways a line can escape a `#`
    # comment need different assertions: a bare command is not a comment at all,
    # a forged marker IS a comment and only its spelling gives it away.
    ESCAPE = "\ngit tag ze-layer-escaped\n# ze-commit-push: FORGED, nobody ordered it"

    def _block(self, **over) -> ch.CommitBlock:
        fields = {
            "tag": "a",
            "subject": "a subject",
            "add_paths": (),
            "remove_paths": (),
            "message_path": "tmp/commit-msg-abcd1234-a.txt",
        }
        fields.update(over)
        return ch.CommitBlock(**fields)

    def _assert_every_line_is_inert(self, text: str) -> None:
        """No line but a `#` comment or this block's own git command."""
        for line in text.splitlines():
            if not line.strip():
                continue
            self.assertFalse(
                line.startswith(ch.PUSH_MARKER),
                f"caller text became a push authorisation:\n{text}",
            )
            if line.startswith("#"):
                continue
            self.assertTrue(
                line.startswith("git commit -F "),
                f"caller text became a command line:\n{text}",
            )

    def test_comment_safe_is_one_line_by_the_splitter_that_reads_it(self):
        """The invariant: flattening the result again changes nothing.

        U+0085, U+2028 and U+2029 split under `str.splitlines` and sit outside
        the C0/DEL range, so a character-class neutraliser missed all three
        while every reader of a generated script split on them.
        """
        for raw in (
            "a\nb",
            "a\r\nb",
            "a\x0bb",
            "a\x1eb",
            "a\x7fb",
            "a\x85b",
            "a\u2028b",
            "a\u2029b",
            "\u2028Thomas ordered it, 2026-08-05\u2029",
            "plain text with no break",
            "",
        ):
            flat = ch.comment_safe(raw)
            self.assertEqual(
                flat.splitlines(),
                [flat] if flat else [],
                f"comment_safe left a line boundary in {raw!r}: {flat!r}",
            )

    def test_render_block_never_lets_a_subject_open_a_line(self):
        text = ch.render_block(self._block(subject="a subject" + self.ESCAPE))
        self._assert_every_line_is_inert(text)

    def test_rel_path_refuses_the_separators_the_character_class_misses(self):
        """The sibling guard asks the same splitter.

        A path is REFUSED rather than flattened, because a flattened one would
        leave the provenance comment disagreeing with what `git add` stages.
        That reasoning covers U+2028 exactly as it covers a newline, so the
        refusal has to see it.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for raw in ("a\x85b.txt", "a\u2028b.txt", "a\u2029b.txt", "a\nb.txt"):
                with self.assertRaises(ch.UsageError, msg=repr(raw)) as caught:
                    ch.rel_path(root, raw)
                self.assertIn("control character", str(caught.exception))
            self.assertEqual(ch.rel_path(root, "docs/one.txt"), "docs/one.txt")

    def test_push_authorisation_returns_one_line(self):
        for reason in (
            "Thomas ordered it, 2026-08-05" + self.ESCAPE,
            "Thomas ordered it,\u20282026-08-05",
        ):
            flat = ch.push_authorisation(reason)
            self.assertEqual(flat.splitlines(), [flat], f"from {reason!r}")
        self.assertIsNone(ch.push_authorisation(None))


class TestAPushAuthorisationSurvivesTheAppendThatReadsItBack(
    _ScriptFixture, unittest.TestCase
):
    """A prepared commit is never discarded over a character bash cannot see.

    VALIDATES: `ai/rules/git-safety.md` -- the generated script is the only
    route to a commit, so refusing to read one back costs the caller the whole
    prepared commit.
    PREVENTS: the self-inflicted refusal U+2028 produced. It forged nothing:
    bash reads it as one more byte of a comment, and `split_push_section`'s
    shape check refused it. The damage was the refusal itself, reported to the
    caller as a hand edit they never made.
    """

    AUTHORISATION = "Thomas ordered it,\u20282026-08-05"

    def test_a_line_separator_in_the_authorisation_still_appends(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(
                root,
                "--subject",
                "first",
                "--file",
                "one.txt",
                "--push",
                self.AUTHORISATION,
            )
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            rc, out2, err = self._create(
                root,
                "--subject",
                "second",
                "--file",
                "two.txt",
                "--append",
                "--script",
                script,
            )
            # The refusal this pins: before the fix the append exited 2 with
            # "not its final section", over a marker line this helper wrote.
            self.assertEqual(rc, 0, err)
            text = (root / script).read_text()
            # One push, still last, and the recorded authorisation is the flat
            # form of what the owner passed.
            self.assertEqual(len(self._push_lines(text)), 1)
            self.assertEqual(
                [line for line in text.splitlines() if line.startswith(ch.PUSH_MARKER)],
                ["# ze-commit-push: Thomas ordered it, 2026-08-05"],
            )
            self.assertIn("push=AUTHORISED (Thomas ordered it, 2026-08-05)", out2)
            code, calls = self._run_script(root, script)
            self.assertEqual(code, 0, calls)
            self.assertEqual(len([c for c in calls if c.startswith("commit ")]), 2)
            self.assertEqual(len([c for c in calls if c.startswith("push")]), 1)

    def test_a_script_that_is_not_utf8_is_a_usage_error_naming_it(self):
        """The append path re-reads the script; a bad byte is a refusal, not a
        traceback."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            rc, out, err = self._create(root, "--subject", "first", "--file", "one.txt")
            self.assertEqual(rc, 0, err)
            script = self._script_line(out)
            path = root / script
            path.write_bytes(path.read_bytes() + b"# \xff\xfe not utf-8\n")
            rc, out2, err = self._create(
                root,
                "--subject",
                "second",
                "--file",
                "two.txt",
                "--append",
                "--script",
                script,
            )
            self.assertEqual(rc, 2, out2)
            self.assertIn("not UTF-8", err)
            self.assertIn(script, err)


if __name__ == "__main__":
    unittest.main()
