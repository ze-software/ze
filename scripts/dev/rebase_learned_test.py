#!/usr/bin/env python3
"""Unit tests for rebase_learned.py (learned-bookkeeping rebase driver).

Two layers:
  * pure decision logic (arg parsing, merge-stage reading) exercised with the
    module's git() global monkeypatched;
  * one real-git test that PROVES the premise the tool's exit code 6 exists for
    -- `git rebase --continue` refusing with a "merge conflicts" message when
    the real cause is unstaged tracked changes. That claim is sourced to git's
    own builtin/rebase.c, which is not vendored here, so it is verified by
    observation rather than cited.
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
import rebase_learned  # noqa: E402


class FakeGit:
    """Stands in for rebase_learned.git(), returning canned CompletedProcess."""

    def __init__(self, stdout="", returncode=0):
        self.stdout = stdout
        self.returncode = returncode
        self.calls = []

    def __call__(self, *args, check=False):
        self.calls.append(args)
        return subprocess.CompletedProcess(args, self.returncode, self.stdout, "")


class TestParseArgs(unittest.TestCase):
    def run_with(self, argv):
        old = sys.argv
        try:
            sys.argv = ["rebase_learned.py", *argv]
            return rebase_learned.parse_args()
        finally:
            sys.argv = old

    def test_defaults_are_all_off(self):
        self.assertEqual(self.run_with([]), (False, [], []))

    def test_flags_collect_repeatable_paths(self):
        accept, theirs, ours = self.run_with(
            [
                "--accept-incoming-delete",
                "--take-theirs",
                "a.md",
                "--take-theirs",
                "b.md",
                "--take-ours",
                "c.md",
            ]
        )
        self.assertTrue(accept)
        self.assertEqual(theirs, ["a.md", "b.md"])
        self.assertEqual(ours, ["c.md"])

    def test_unknown_arg_exits_2(self):
        # Exit 2 is the documented "git command failed unexpectedly" / usage code.
        with self.assertRaises(SystemExit) as cm:
            self.run_with(["--wat"])
        self.assertEqual(cm.exception.code, 2)

    def test_help_exits_0(self):
        with self.assertRaises(SystemExit) as cm:
            self.run_with(["--help"])
        self.assertEqual(cm.exception.code, 0)


class TestMergeStages(unittest.TestCase):
    # `git ls-files -u` format: <mode> <sha> <stage>\t<path>
    BOTH = "100644 aaaa 1\tspec.md\n100644 bbbb 2\tspec.md\n100644 cccc 3\tspec.md\n"
    INCOMING_DELETE = "100644 aaaa 1\tspec.md\n100644 bbbb 2\tspec.md\n"
    OURS_DELETE = "100644 aaaa 1\tspec.md\n100644 cccc 3\tspec.md\n"

    def with_git(self, stdout, fn):
        old = rebase_learned.git
        try:
            rebase_learned.git = FakeGit(stdout)
            return fn()
        finally:
            rebase_learned.git = old

    def test_stages_parsed_from_ls_files_output(self):
        got = self.with_git(
            self.BOTH, lambda: rebase_learned.conflict_stages("spec.md")
        )
        self.assertEqual(got, {1, 2, 3})

    def test_incoming_delete_is_stage2_without_stage3(self):
        # The replayed commit removed the file; ours still has it.
        got = self.with_git(
            self.INCOMING_DELETE, lambda: rebase_learned.is_incoming_delete("spec.md")
        )
        self.assertTrue(got)

    def test_content_conflict_is_not_an_incoming_delete(self):
        got = self.with_git(
            self.BOTH, lambda: rebase_learned.is_incoming_delete("spec.md")
        )
        self.assertFalse(got)

    def test_our_side_deleted_is_not_an_incoming_delete(self):
        got = self.with_git(
            self.OURS_DELETE, lambda: rebase_learned.is_incoming_delete("spec.md")
        )
        self.assertFalse(got)


def _git(repo, *args, env=None):
    return subprocess.run(
        ["git", *args], cwd=repo, capture_output=True, text=True, env=env
    )


class TestRebaseContinueMessageIsMisleading(unittest.TestCase):
    """Observation test for the premise behind exit code 6.

    rebase_learned.py claims `git rebase --continue` refuses whenever unstaged
    tracked changes exist -- not only when index entries are unmerged -- while
    printing a message about merge conflicts. That is sourced to git's
    builtin/rebase.c (ACTION_CONTINUE -> has_unstaged_changes()), which this
    repo does not vendor, so prove it against the installed git instead.
    """

    def test_continue_blocked_by_unstaged_change_with_zero_unmerged(self):
        env = dict(os.environ, GIT_EDITOR="true", GIT_SEQUENCE_EDITOR="true")
        with tempfile.TemporaryDirectory() as repo:
            r = _git(repo, "init", "-b", "main", env=env)
            if r.returncode != 0:
                self.skipTest(f"git init unavailable: {r.stderr}")
            _git(repo, "config", "user.email", "t@example.com", env=env)
            _git(repo, "config", "user.name", "Test", env=env)
            # Throwaway fixture repo: never our own commits, so signing is off.
            _git(repo, "config", "commit.gpgsign", "false", env=env)

            p = Path(repo)
            (p / "conflicted.txt").write_text("base\n")
            (p / "bystander.txt").write_text("untouched\n")
            _git(repo, "add", "-A", env=env)
            _git(repo, "commit", "-m", "base", env=env)

            _git(repo, "checkout", "-b", "feature", env=env)
            (p / "conflicted.txt").write_text("feature\n")
            _git(repo, "commit", "-am", "feature", env=env)

            _git(repo, "checkout", "main", env=env)
            (p / "conflicted.txt").write_text("mainline\n")
            _git(repo, "commit", "-am", "mainline", env=env)

            _git(repo, "checkout", "feature", env=env)
            r = _git(repo, "rebase", "main", env=env)
            self.assertNotEqual(r.returncode, 0, "expected the rebase to conflict")

            # Resolve the conflict properly and stage it: zero unmerged entries.
            (p / "conflicted.txt").write_text("resolved\n")
            _git(repo, "add", "conflicted.txt", env=env)
            unmerged = _git(repo, "ls-files", "-u", env=env).stdout.strip()
            self.assertEqual(unmerged, "", "precondition: nothing should be unmerged")

            # Now dirty an UNRELATED tracked file, leaving it unstaged.
            (p / "bystander.txt").write_text("unstaged edit\n")

            r = _git(repo, "rebase", "--continue", env=env)
            out = (r.stdout + r.stderr).lower()

            self.assertNotEqual(
                r.returncode, 0, "continue should refuse while unstaged changes exist"
            )
            # THE LIE: the refusal never names the real blocker. Asserting the
            # absence of the cause rather than git's exact phrasing keeps this
            # robust across git versions while still pinning the behaviour the
            # tool exists to explain. Observed verbatim on git 2.55.0:
            #   "You must edit all merge conflicts and then
            #    mark them as resolved using git add"
            self.assertNotIn(
                "bystander", out, f"git named the real cause after all: {out!r}"
            )
            self.assertIn("merge conflicts", out, f"unexpected refusal text: {out!r}")

            # ...and the tool's own detector IS able to name it, which is the
            # whole value of exit code 6.
            self.assertIn(
                "bystander.txt", _git(repo, "diff", "--name-only", env=env).stdout
            )

            _git(repo, "rebase", "--abort", env=env)


if __name__ == "__main__":
    unittest.main()
