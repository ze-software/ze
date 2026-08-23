#!/usr/bin/env python3
"""Unit tests for scripts/dev/verify_worktree.py, the commit-scoped gate runner.

These are mostly about one property: a red run's stage logs MUST outlive the
worktree they were written in. The gate writes them to `tmp/verify` inside the
tree it runs in (`stageLogDir`, scripts/status/verify_run.go), and the runner
removes that tree on every exit path, so the reason a 25-to-53-minute run went
red used to be destroyed before anyone could read it. The end-to-end test below
is the discriminating one: delete the `save_logs` call and it fails, because the
log content is gone once the worktree is removed.

The green case is asserted from the other side. Saving an hour of stage output
nobody reads is the failure this would turn into if "save the logs" were applied
without the red/green distinction.
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import verify_worktree  # noqa: E402

SCRIPT = Path(__file__).resolve().parent / "verify_worktree.py"

# A make target that writes a stage log and then fails, which is the shape the
# real gate has when a stage goes red: the logs exist and the exit code is not
# zero. `RED_MARKER` is what a reader would have lost.
RED_MARKER = "stage nine said the tier check failed"

MAKEFILE = f"""\
red:
\t@mkdir -p tmp/verify
\t@echo '{RED_MARKER}' > tmp/verify/stage.log
\t@exit 1

green:
\t@mkdir -p tmp/verify
\t@echo 'nothing worth keeping' > tmp/verify/stage.log
\t@exit 0

early-red:
\t@exit 1
"""


class Repo:
    """A throwaway git repo the runner can add a worktree to."""

    def __init__(self, root: Path) -> None:
        self.root = root
        # The real repo gitignores tmp/, and both the worktree and the saved
        # logs live there. Without this the worktree add would refuse.
        (root / ".gitignore").write_text("tmp/\n", encoding="utf-8")
        (root / "Makefile").write_text(MAKEFILE, encoding="utf-8")
        self.git("init", "-q")
        self.git("config", "user.email", "t@example.com")
        self.git("config", "user.name", "T")
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "c", "--no-gpg-sign")

    def git(self, *args: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            ["git", *args], cwd=self.root, capture_output=True, text=True, check=False
        )

    def run_runner(self, target: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(SCRIPT), "--target", target],
            cwd=self.root,
            capture_output=True,
            text=True,
            check=False,
        )

    def saved_logs(self) -> list[Path]:
        base = self.root / "tmp" / "verify-worktree-logs"
        return sorted(p for p in base.iterdir()) if base.is_dir() else []

    def live_worktrees(self) -> list[Path]:
        base = self.root / "tmp" / "verify-worktree"
        return sorted(p for p in base.iterdir()) if base.is_dir() else []


class SaveLogsTest(unittest.TestCase):
    """`save_logs` in isolation: what it copies, and what it declines to."""

    def test_copies_the_stage_log_directory(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            worktree = root / "wt"
            (worktree / "tmp" / "verify").mkdir(parents=True)
            (worktree / "tmp" / "verify" / "stage.log").write_text(
                RED_MARKER, encoding="utf-8"
            )

            dest = verify_worktree.save_logs(root, worktree, "run-1")

            self.assertIsNotNone(dest)
            assert dest is not None
            self.assertEqual(
                RED_MARKER, (dest / "stage.log").read_text(encoding="utf-8")
            )

    def test_returns_none_when_the_gate_wrote_no_logs(self) -> None:
        """A gate that died before its first stage has nothing to save.

        The caller prints a different line for this, so the two cases must stay
        distinguishable. Returning an empty directory would read as "logs saved"
        and send a reader to an empty path.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            worktree = root / "wt"
            (worktree / "tmp").mkdir(parents=True)

            self.assertIsNone(verify_worktree.save_logs(root, worktree, "run-1"))
            self.assertFalse((root / "tmp" / "verify-worktree-logs").exists())

    def test_a_second_run_replaces_the_first(self) -> None:
        """Two runs of one commit in the same second must not merge their logs.

        The destination name is stamp plus short sha, so a rerun can land on an
        existing directory. Merging would leave a stage file from the earlier
        run beside the later one with nothing saying which is which.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            worktree = root / "wt"
            (worktree / "tmp" / "verify").mkdir(parents=True)
            (worktree / "tmp" / "verify" / "first.log").write_text(
                "a", encoding="utf-8"
            )
            verify_worktree.save_logs(root, worktree, "run-1")

            (worktree / "tmp" / "verify" / "first.log").unlink()
            (worktree / "tmp" / "verify" / "second.log").write_text(
                "b", encoding="utf-8"
            )
            dest = verify_worktree.save_logs(root, worktree, "run-1")

            assert dest is not None
            self.assertEqual(["second.log"], [p.name for p in sorted(dest.iterdir())])


class TimestampedPathTest(unittest.TestCase):
    """The worktree path must stay NEW per run, and this asserts it.

    The property is load-bearing for a reason nobody chose it for: a fresh
    absolute path misses the `go test` result cache, so a gate run here cannot
    clear a verification-debt row on a verdict from a run that never happened.
    The obvious future optimisation is to key the path on the sha alone and
    reuse the checkout, which is right about the cost and wrong about the
    consequence. A comment reaches whoever reads that line. This reaches
    whoever changes it.
    """

    def test_two_runs_at_one_commit_get_different_paths(self) -> None:
        root = Path("/repo")
        sha = "d3bd1d88b88c0000000000000000000000000000"

        first = verify_worktree.worktree_path(root, sha, "20260822T130818Z")
        second = verify_worktree.worktree_path(root, sha, "20260822T142530Z")

        self.assertNotEqual(
            first,
            second,
            "the worktree path stopped varying per run, which re-arms the "
            "stale go test cache against the debt ledger",
        )

    def test_the_path_carries_the_commit_so_a_reader_can_attribute_it(
        self,
    ) -> None:
        root = Path("/repo")
        sha = "d3bd1d88b88c0000000000000000000000000000"

        path = verify_worktree.worktree_path(root, sha, "20260822T130818Z")

        self.assertIn("d3bd1d88b88c", path.name)
        self.assertIn("20260822T130818Z", path.name)
        self.assertEqual(root / "tmp" / "verify-worktree", path.parent)


class RunnerEndToEndTest(unittest.TestCase):
    """The whole runner, against a real git worktree and a real make target."""

    def test_a_red_run_leaves_its_logs_behind_after_the_worktree_is_gone(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Repo(Path(tmp))

            result = repo.run_runner("red")

            # make's own exit code, whatever it is; the runner passes it
            # through and this test is about the logs, not the number.
            self.assertNotEqual(0, result.returncode, result.stdout + result.stderr)
            self.assertEqual([], repo.live_worktrees(), "worktree was not removed")
            saved = repo.saved_logs()
            self.assertEqual(1, len(saved), f"expected one saved log dir, got {saved}")
            self.assertEqual(
                RED_MARKER,
                (saved[0] / "stage.log").read_text(encoding="utf-8").strip(),
            )
            self.assertIn("logs saved to", result.stdout)

    def test_a_green_run_saves_nothing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Repo(Path(tmp))

            result = repo.run_runner("green")

            self.assertEqual(0, result.returncode, result.stdout + result.stderr)
            self.assertEqual([], repo.saved_logs())

    def test_a_run_that_died_before_its_first_stage_says_so(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Repo(Path(tmp))

            result = repo.run_runner("early-red")

            self.assertNotEqual(0, result.returncode)
            self.assertEqual([], repo.saved_logs())
            self.assertIn("wrote no stage logs", result.stdout)


class SweepAbandonedTest(unittest.TestCase):
    """A killed run leaks its whole checkout, and the next run reclaims it.

    Neither the exit paths nor the `finally` run on SIGKILL, which cannot be
    trapped, so the leak is real and unavoidable at the point it happens. One
    measured on 2026-08-23 held 14G for five and a half hours on a volume that
    reached 100 percent twice that day.
    """

    def _base(self, repo: Repo) -> Path:
        base = repo.root / "tmp" / "verify-worktree"
        base.mkdir(parents=True, exist_ok=True)
        return base

    def _abandoned(self, repo: Repo, name: str) -> Path:
        """A worktree whose owner pid is dead, as a killed run leaves one."""
        path = self._base(repo) / name
        repo.git("worktree", "add", "--detach", str(path), "HEAD")
        # Pid 0 is never a live user process, so this stands in for an owner
        # that has gone. Writing a real dead pid would race with reuse.
        verify_worktree.owner_marker(path).write_text("0\n", encoding="utf-8")
        return path

    def test_an_abandoned_worktree_is_removed(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Repo(Path(tmp))
            path = self._abandoned(repo, "20260822T203132Z-deadbeef1234")

            removed = verify_worktree.sweep_abandoned(repo.root)

            self.assertEqual([path], removed)
            self.assertFalse(path.exists(), "the leaked checkout is still there")
            self.assertFalse(verify_worktree.owner_marker(path).exists())

    def test_a_live_owner_is_left_alone(self) -> None:
        """The running gate's own worktree must survive a concurrent sweep."""
        with tempfile.TemporaryDirectory() as tmp:
            repo = Repo(Path(tmp))
            path = self._abandoned(repo, "20260822T203132Z-livelive1234")
            verify_worktree.owner_marker(path).write_text(
                f"{os.getpid()}\n", encoding="utf-8"
            )

            self.assertEqual([], verify_worktree.sweep_abandoned(repo.root))
            self.assertTrue(path.exists(), "a live run's worktree was swept")

    def test_uncommitted_work_is_never_swept(self) -> None:
        """never-destroy-work outranks reclaiming disk. A leftover is
        disposable; somebody's unfinished work inside one is not."""
        with tempfile.TemporaryDirectory() as tmp:
            repo = Repo(Path(tmp))
            path = self._abandoned(repo, "20260822T203132Z-dirtydirty12")
            (path / "someone-was-working.txt").write_text("keep me\n")

            removed = verify_worktree.sweep_abandoned(repo.root)

            self.assertEqual([], removed)
            self.assertTrue(path.exists(), "uncommitted work was destroyed")
            self.assertEqual(
                "keep me\n", (path / "someone-was-working.txt").read_text()
            )

    def test_the_marker_does_not_make_its_own_worktree_look_dirty(self) -> None:
        """The marker lives beside the worktree for exactly this reason. Inside
        it, git reports it as untracked and nothing is ever swept."""
        with tempfile.TemporaryDirectory() as tmp:
            repo = Repo(Path(tmp))
            path = self._abandoned(repo, "20260822T203132Z-cleanclean12")

            dirty = subprocess.run(
                ["git", "-C", str(path), "status", "--porcelain"],
                capture_output=True,
                text=True,
                check=False,
            )

            self.assertEqual("", dirty.stdout.strip(), "the marker dirtied the tree")

    def test_a_run_sweeps_before_it_builds(self) -> None:
        """End to end: the leftover is gone and the new worktree is not."""
        with tempfile.TemporaryDirectory() as tmp:
            repo = Repo(Path(tmp))
            leaked = self._abandoned(repo, "20260101T000000Z-000000000000")

            repo.run_runner("green")

            self.assertFalse(leaked.exists(), "the leftover survived a later run")


if __name__ == "__main__":
    unittest.main()
