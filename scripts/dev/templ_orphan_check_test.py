#!/usr/bin/env python3
"""Unit tests for templ_orphan_check.py, the only sight of an orphan ze has.

`make ze-templ-generate-check` passes -keep-orphaned-files, so templ neither
deletes nor reports a *_templ.go whose .templ source is gone. This script is
what reports it, and a stale generated file passes every other gate.

Two directions are tested, because both are defects. A real orphan on disk
must red. A pair deleted together in the worktree must NOT red: git still
lists the tracked half, and the file the check would name is already gone.
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from templ_orphan_check import (  # noqa: E402
    main,
    orphan_outputs,
    repo_paths,
    stray_sources,
)

SCRIPT = Path(__file__).resolve().parent / "templ_orphan_check.py"


def _git(root: Path, *args: str) -> None:
    subprocess.run(["git", *args], cwd=root, check=True, capture_output=True, text=True)


def _repo(root: Path) -> None:
    _git(root, "init", "-q")
    _git(root, "config", "user.email", "t@example.com")
    _git(root, "config", "user.name", "t")
    _git(root, "config", "commit.gpgsign", "false")
    # Pin the quoting the non-ASCII test exists to exercise. git quotes a
    # non-ASCII path only when core.quotePath is true, and that is the default.
    # A developer who sets it false globally would otherwise pass that test with
    # the NUL split removed, so the fixture states the setting it depends on.
    _git(root, "config", "core.quotePath", "true")


def _write(root: Path, rel: str, text: str = "x\n") -> Path:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    return path


def _run(root: Path) -> subprocess.CompletedProcess:
    """Run the check the way the make recipe runs it."""
    return subprocess.run(
        [sys.executable, str(SCRIPT), "--root", str(root)],
        capture_output=True,
        text=True,
    )


class TestOrphanCheck(unittest.TestCase):
    def test_untracked_orphan_reds_and_survives(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            orphan = _write(root, "internal/a/b/page_templ.go", "package b\n")

            got = _run(root)

            self.assertNotEqual(got.returncode, 0, got.stdout + got.stderr)
            self.assertIn("internal/a/b/page_templ.go", got.stdout)
            self.assertTrue(orphan.exists(), "the check deleted the file it reported")

    def test_committed_orphan_reds_and_survives(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            orphan = _write(root, "internal/a/b/page_templ.go", "package b\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-qm", "init")

            got = _run(root)

            self.assertNotEqual(got.returncode, 0, got.stdout + got.stderr)
            self.assertIn("internal/a/b/page_templ.go", got.stdout)
            self.assertTrue(orphan.exists(), "the check deleted the file it reported")

    def test_non_ascii_orphan_reds(self):
        """git quotes such a path unless -z is passed, and the quote hides it."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            orphan = _write(root, "internal/a/café_templ.go", "package a\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-qm", "init")

            got = _run(root)

            self.assertNotEqual(got.returncode, 0, got.stdout + got.stderr)
            self.assertIn("café_templ.go", got.stdout)
            self.assertTrue(orphan.exists(), "the check deleted the file it reported")

    def test_pair_deleted_together_in_the_worktree_passes(self):
        """The routine delete flow: remove both files, do not stage, run.

        git ls-files still lists the tracked _templ.go, so a check that reads
        the index alone reds over a file that is already gone.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            source = _write(root, "internal/a/b/page.templ", "package b\n")
            generated = _write(root, "internal/a/b/page_templ.go", "package b\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-qm", "init")
            source.unlink()
            generated.unlink()

            got = _run(root)

            self.assertEqual(got.returncode, 0, got.stdout + got.stderr)

    def test_stray_deleted_in_the_worktree_passes(self):
        """Same population defect, on the other predicate."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            stray = _write(root, "cmd/ze/page.templ", "package main\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-qm", "init")
            stray.unlink()

            got = _run(root)

            self.assertEqual(got.returncode, 0, got.stdout + got.stderr)

    def test_paired_source_and_output_pass(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            _write(root, "internal/a/b/page.templ", "package b\n")
            generated = _write(root, "internal/a/b/page_templ.go", "package b\n")

            got = _run(root)

            self.assertEqual(got.returncode, 0, got.stdout + got.stderr)
            self.assertTrue(generated.exists())

    def test_stray_templ_outside_the_scope_reds(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            stray = _write(root, "cmd/ze/page.templ", "package main\n")

            got = _run(root)

            self.assertNotEqual(got.returncode, 0, got.stdout + got.stderr)
            self.assertIn("cmd/ze/page.templ", got.stdout)
            self.assertTrue(stray.exists())

    def test_vendored_templ_is_the_dependency_and_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            _write(root, "vendor/github.com/a-h/templ/x/list.templ", "package x\n")

            got = _run(root)

            self.assertEqual(got.returncode, 0, got.stdout + got.stderr)

    def test_ignored_orphan_is_out_of_scope(self):
        """An ignored file cannot be committed, so nothing can lose it."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            _write(root, ".gitignore", "internal/a/\n")
            _write(root, "internal/a/b/page_templ.go", "package b\n")

            got = _run(root)

            self.assertEqual(got.returncode, 0, got.stdout + got.stderr)

    def test_main_returns_zero_on_an_empty_tree(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)

            self.assertEqual(main(["--root", str(root)]), 0)

    def test_predicates_take_paths_that_exist(self):
        """Both predicates read `repo_paths` output, which is filtered to disk."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            paths = [
                "internal/a/kept.templ",
                "internal/a/kept_templ.go",
                "internal/a/gone_templ.go",
                "internal/a/page.go",
                "vendor/x/list.templ",
                "cmd/ze/stray.templ",
            ]
            for p in paths:
                _write(root, p)

            self.assertEqual(stray_sources(paths), ["cmd/ze/stray.templ"])
            self.assertEqual(orphan_outputs(root, paths), ["internal/a/gone_templ.go"])

    def test_repo_paths_drops_a_tracked_file_the_worktree_lost(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _repo(root)
            kept = _write(root, "internal/a/kept.templ")
            _write(root, "internal/a/gone.templ")
            _git(root, "add", "-A")
            _git(root, "commit", "-qm", "init")
            (root / "internal/a/gone.templ").unlink()

            self.assertEqual(repo_paths(root), [str(kept.relative_to(root))])


if __name__ == "__main__":
    unittest.main()
