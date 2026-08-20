#!/usr/bin/env python3
"""Unit tests for scripts/dev/verify-status.sh, the last-verify fingerprint.

The scoped `check PATH...` mode is what these mostly cover, because it decides
whether a session is allowed to hold evidence about its own code. Several
sessions share this checkout and it routinely carries 300+ uncommitted files, so
the unscoped whole-tree answer is STALE within seconds of a PASS -- and almost
always for a file the asking session never touched. A scoped answer that leaked
a concurrent edit would put the old behaviour back silently, so the isolation is
asserted from both sides: my path stays FRESH, their path goes STALE.
"""

from __future__ import annotations

import os
import subprocess
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "verify-status.sh"


class VerifyStatusRepo:
    """A throwaway git repo with the script available under scripts/dev/."""

    def __init__(self, root: Path) -> None:
        self.root = root
        (root / "scripts" / "dev").mkdir(parents=True)
        (root / "scripts" / "dev" / "verify-status.sh").write_bytes(SCRIPT.read_bytes())
        os.chmod(root / "scripts" / "dev" / "verify-status.sh", 0o755)
        # The real repo gitignores tmp/, and the status file plus the manifest
        # both live there. Without this the fingerprint would hash its own
        # output and no tree could ever read as unchanged.
        (root / ".gitignore").write_text("tmp/\n", encoding="utf-8")
        self.git("init", "-q")
        self.git("config", "user.email", "t@example.com")
        self.git("config", "user.name", "T")

    def git(self, *args: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            ["git", *args], cwd=self.root, capture_output=True, text=True, check=False
        )

    def write(self, rel: str, text: str) -> None:
        path = self.root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")

    def commit_all(self, message: str = "c") -> None:
        self.git("add", "-A")
        self.git("commit", "-q", "--no-gpg-sign", "-m", message)

    def run(self, *args: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            ["bash", "scripts/dev/verify-status.sh", *args],
            cwd=self.root,
            capture_output=True,
            text=True,
            check=False,
        )


class TestScopedCheck(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.repo = VerifyStatusRepo(Path(self._tmp.name))
        self.repo.write("mine.go", "package a\n")
        self.repo.write("theirs.go", "package b\n")
        self.repo.commit_all()
        self.assertEqual(
            self.repo.run("write", "0", "ze-precommit-verify").returncode, 0
        )

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_an_unscoped_check_is_fresh_on_an_untouched_tree(self) -> None:
        result = self.repo.run("check")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("FRESH", result.stdout)

    def test_a_concurrent_edit_elsewhere_leaves_my_path_fresh(self) -> None:
        self.repo.write("theirs.go", "package b\n// another session\n")
        self.assertEqual(
            self.repo.run("check").returncode, 1, "whole tree must go stale"
        )
        result = self.repo.run("check", "mine.go")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("FRESH", result.stdout)

    def test_an_edit_to_the_scoped_path_is_still_stale(self) -> None:
        self.repo.write("mine.go", "package a\n// edited after the pass\n")
        result = self.repo.run("check", "mine.go")
        self.assertEqual(result.returncode, 1, result.stdout)
        self.assertIn("STALE", result.stdout)

    def test_a_directory_scopes_to_everything_under_it(self) -> None:
        self.repo.write("pkg/deep/x.go", "package d\n")
        self.repo.commit_all()
        self.repo.run("write", "0", "ze-precommit-verify")
        self.repo.write("pkg/deep/x.go", "package d\n// touched\n")
        self.assertEqual(self.repo.run("check", "pkg").returncode, 1)
        self.assertEqual(self.repo.run("check", "mine.go").returncode, 0)

    def test_a_new_untracked_file_makes_its_own_scope_stale(self) -> None:
        """A file absent at PASS time appears in neither manifest by path, so a
        comparison that only walked the STORED rows would call this fresh."""
        self.repo.write("mine_extra.go", "package a\n")
        self.assertEqual(self.repo.run("check", "mine_extra.go").returncode, 1)

    def test_a_moved_head_is_stale_even_when_the_file_is_untouched(self) -> None:
        """The working file can equal HEAD while HEAD itself changed underneath,
        which changes what was verified."""
        self.repo.write("other.go", "package c\n")
        self.repo.commit_all("second")
        result = self.repo.run("check", "mine.go")
        self.assertEqual(result.returncode, 1, result.stdout)
        self.assertIn("HEAD moved", result.stdout)

    def test_check_scoped_ignores_unnamed_paths(self) -> None:
        """The compare reads the named paths' rows and nothing else.

        Asserted in both directions, because a scope that leaked would be
        invisible from one alone: an unnamed path that GAINS a change must not
        turn my answer STALE, and an unnamed path that LOSES one must not turn
        somebody else's STALE answer FRESH.
        """
        self.repo.write("theirs.go", "package b\n// their edit\n")
        result = self.repo.run("check", "mine.go")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("1 scoped path", result.stdout)

        self.repo.write("mine.go", "package a\n// my edit\n")
        self.repo.write("theirs.go", "package b\n")
        result = self.repo.run("check", "mine.go")
        self.assertEqual(result.returncode, 1, result.stdout)
        self.assertIn("STALE", result.stdout)

    def test_a_shell_metacharacter_in_a_path_is_matched_literally(self) -> None:
        """Paths travel as argv and are compared with awk's `==` and `index()`,
        so `;`, `$` and `*` are ordinary characters. A path that globbed or
        substituted would compare some OTHER path's rows, and the wrong answer
        can fall either way."""
        self.repo.write("we$ird;name.go", "package w\n")
        self.repo.write("mine*.go", "package s\n")
        self.repo.commit_all("metachars")
        self.assertEqual(
            self.repo.run("write", "0", "ze-precommit-verify").returncode, 0
        )
        self.repo.write("we$ird;name.go", "package w\n// edited\n")
        self.repo.write("mine.go", "package a\n// edited\n")

        result = self.repo.run("check", "we$ird;name.go")
        self.assertEqual(result.returncode, 1, result.stdout)
        self.assertIn("STALE", result.stdout)

        # `mine*.go` is a real file here and it is untouched. Globbing would
        # match the edited `mine.go` and report STALE for a file the caller
        # never named.
        result = self.repo.run("check", "mine*.go")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("FRESH", result.stdout)

        # Nothing in a path was ever executed.
        self.assertEqual(
            sorted(p.name for p in self.repo.root.glob("*.go")),
            ["mine*.go", "mine.go", "theirs.go", "we$ird;name.go"],
        )

    def test_a_failed_pass_is_never_fresh_for_any_scope(self) -> None:
        self.repo.run("write", "1", "ze-precommit-verify")
        self.assertEqual(self.repo.run("check", "mine.go").returncode, 1)

    def test_a_skipped_suite_pass_is_never_fresh_for_any_scope(self) -> None:
        env = dict(os.environ, ZE_SKIP_SUITES="unit")
        subprocess.run(
            ["bash", "scripts/dev/verify-status.sh", "write", "0"],
            cwd=self.repo.root,
            capture_output=True,
            text=True,
            env=env,
            check=False,
        )
        self.assertEqual(self.repo.run("check", "mine.go").returncode, 1)


if __name__ == "__main__":
    unittest.main()
