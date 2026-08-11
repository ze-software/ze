#!/usr/bin/env python3
"""Unit tests for relax-census.py, the `test-relax:` ceiling gate.

(Named relax_census_test.py rather than test_relax_census.py to match the
scripts/dev convention; picked up by the *_test.py glob in python_tests_test.go
either way.)

Driven end-to-end through the real entry point (subprocess), per the test
corollary in ai/rules/evidence.md: calling census() directly proves the counter
counts, not that the gate refuses anything. The exit code IS the gate.

Each fixture repo symlinks the real scripts/dev/audit-test-relaxation.py into
place, because the census loads its token reader from there and exits 2 when it
cannot. Without the symlink every test would pass on an import failure instead of
on the behaviour it names.

The central property, the same one the audit has: a pass must mean "I counted and
the count is under the ceiling", never "I counted nothing".
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO_ROOT = HERE.parent.parent
SCRIPT = HERE / "relax-census.py"
AUDIT = HERE / "audit-test-relaxation.py"
HOOK = REPO_ROOT / ".claude" / "hooks" / "pretool-writeedit.py"
HOOK_LIB = REPO_ROOT / ".claude" / "hooks" / "lib"

TOKEN_GO = "// test-relax: {}\n"
TOKEN_CI = "# // test-relax: {}\n"


class Fixture:
    """A throwaway git repo carrying a known number of tokens."""

    def __init__(self, tokens_go=0, tokens_ci=0, ceiling=None, extra=None):
        self.dir = Path(tempfile.mkdtemp(prefix="ze-census-"))
        self._git("init", "-q")
        (self.dir / "scripts" / "dev").mkdir(parents=True, exist_ok=True)
        os.symlink(AUDIT, self.dir / "scripts" / "dev" / "audit-test-relaxation.py")
        # The hook too: the census reads what makes two `raised-for:` reasons the
        # same from `_reason_key` there, and REFUSES when it cannot. Without the
        # symlink every test would assert an import failure rather than behaviour.
        (self.dir / ".claude" / "hooks").mkdir(parents=True, exist_ok=True)
        os.symlink(HOOK, self.dir / ".claude" / "hooks" / "pretool-writeedit.py")
        os.symlink(HOOK_LIB, self.dir / ".claude" / "hooks" / "lib")

        for i in range(tokens_go):
            self.write(f"pkg/a{i}_test.go", TOKEN_GO.format(f"go reason {i}"))
        for i in range(tokens_ci):
            self.write(f"test/plugin/b{i}.ci", TOKEN_CI.format(f"ci reason {i}"))
        for rel, body in (extra or {}).items():
            self.write(rel, body)
        if ceiling is not None:
            self.write("test/relax-ceiling.txt", f"# prose\n{ceiling}\n")
        self.commit()

    def _git(self, *a):
        return subprocess.run(["git", *a], cwd=self.dir, capture_output=True, text=True)

    def write(self, rel, body):
        full = self.dir / rel
        full.parent.mkdir(parents=True, exist_ok=True)
        full.write_text(body, encoding="utf-8")

    def commit(self):
        self._git("add", "-A")
        self._git(
            "-c",
            "user.email=t@t",
            "-c",
            "user.name=t",
            "-c",
            "commit.gpgsign=false",
            "commit",
            "-q",
            "-m",
            "b",
        )

    def run(self, *args):
        return subprocess.run(
            [sys.executable, str(SCRIPT), *args],
            cwd=self.dir,
            capture_output=True,
            text=True,
        )


class TestCeilingIsEnforced(unittest.TestCase):
    def test_over_the_ceiling_fails(self):
        r = Fixture(tokens_go=3, ceiling=2).run()
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("OVER THE CEILING", r.stderr)

    def test_at_the_ceiling_passes(self):
        r = Fixture(tokens_go=2, ceiling=2).run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("2 token(s)", r.stdout)

    def test_under_the_ceiling_passes(self):
        r = Fixture(tokens_go=1, tokens_ci=1, ceiling=9).run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("2 token(s)", r.stdout)


class TestAMissingCeilingIsNeverAPass(unittest.TestCase):
    """The zero-value trap the audit already had once: no ceiling must not read as
    'nothing to enforce'. Deleting the file would otherwise be the cheapest route
    from red to green, cheaper than fixing a single test."""

    def test_missing_ceiling_file_refuses(self):
        r = Fixture(tokens_go=5, ceiling=None).run()
        self.assertEqual(r.returncode, 2, r.stdout + r.stderr)
        self.assertIn("relax-ceiling.txt", r.stderr)

    def test_ceiling_without_an_integer_refuses(self):
        f = Fixture(tokens_go=1, ceiling=1)
        f.write("test/relax-ceiling.txt", "# prose only, no number\n")
        f.commit()
        r = f.run()
        self.assertEqual(r.returncode, 2, r.stdout + r.stderr)
        # The MESSAGE, not just the code: a missing hook symlink also exits 2, so a
        # code-only assertion passes on an import failure instead of on this branch.
        self.assertIn("no bare integer", r.stderr)


class TestWhatCounts(unittest.TestCase):
    def test_vendor_is_excluded(self):
        f = Fixture(
            tokens_go=1,
            ceiling=1,
            extra={"vendor/v_test.go": TOKEN_GO.format("not ours")},
        )
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("1 token(s)", r.stdout)

    def test_a_ci_outside_test_is_not_a_test(self):
        f = Fixture(
            tokens_go=1,
            ceiling=1,
            extra={"docs/sample.ci": TOKEN_CI.format("documentation sample")},
        )
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("1 token(s)", r.stdout)

    def test_untracked_drafts_do_not_red_the_gate(self):
        """A draft is not in the repository yet. Reddening the gate for one is how
        a gate gets switched off."""
        f = Fixture(tokens_go=1, ceiling=1)
        f.write("pkg/draft_test.go", TOKEN_GO.format("uncommitted draft"))
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_an_uncommitted_edit_to_a_tracked_test_does_not_red_the_gate(self):
        """The case that actually arises in this repository.

        Several sessions share one checkout, so the working tree carries other
        people's half-finished work at all times. Counting it made the gate red on
        edits its author never made: measured 751 -> 752 -> 755 within an hour on
        2026-08-10, on three sessions that had never touched this gate. The census
        reads HEAD for exactly this reason.
        """
        f = Fixture(tokens_go=1, ceiling=1)
        f.write(
            "pkg/a0_test.go",
            TOKEN_GO.format("go reason 0")
            + TOKEN_GO.format("a second one, uncommitted"),
        )
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("1 token(s)", r.stdout)
        # ...but it is still SHOWN, or the session cannot see what it is about to land.
        self.assertIn("working tree: 2", r.stdout)

    def test_worktree_flag_counts_the_working_tree(self):
        f = Fixture(tokens_go=1, ceiling=1)
        f.write(
            "pkg/a0_test.go", TOKEN_GO.format("go reason 0") + TOKEN_GO.format("second")
        )
        r = f.run("--worktree")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("2 token(s)", r.stderr)

    def test_both_carrier_syntaxes_count(self):
        f = Fixture(
            ceiling=3,
            extra={
                "pkg/a_test.go": "// test-relax: go form\n",
                "test/p/b.ci": "# // test-relax: legacy nested form\n",
                "test/p/c.ci": "# test-relax: natural hash form\n",
            },
        )
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("3 token(s)", r.stdout)


class TestReasonsAreReportedWhole(unittest.TestCase):
    """A first-line-only capture is what made the 2026-08-10 corpus unreviewable:
    63% of its reasons ran past one line."""

    def test_list_prints_the_whole_justification(self):
        f = Fixture(
            ceiling=1,
            extra={
                "pkg/a_test.go": (
                    "// test-relax: TestNarrowTS covered narrowTS, a function\n"
                    "// with no non-test caller, deleted in this change.\n"
                    "func TestX(t *testing.T) {}\n"
                )
            },
        )
        r = f.run("--list")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("no non-test caller", r.stdout)
        self.assertIn("TestNarrowTS", r.stdout)


class TestLowerOnlyLowers(unittest.TestCase):
    def test_lower_rewrites_the_ceiling_down(self):
        f = Fixture(tokens_go=2, ceiling=9)
        r = f.run("--lower")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("9 -> 2", r.stdout)
        self.assertIn("\n2\n", (f.dir / "test" / "relax-ceiling.txt").read_text())

    def test_lower_never_raises(self):
        f = Fixture(tokens_go=5, ceiling=2)
        r = f.run("--lower")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        body = (f.dir / "test" / "relax-ceiling.txt").read_text()
        self.assertIn("\n2\n", body)
        self.assertNotIn("\n5\n", body)


class TestAPartialReadIsNeverAPass(unittest.TestCase):
    """ "Clean" must mean "I counted everything and it is under the ceiling", never
    "I counted what I could open" (ai/rules/evidence.md)."""

    def test_an_unreadable_tracked_test_refuses(self):
        f = Fixture(tokens_go=3, ceiling=2)
        self.assertEqual(f.run().returncode, 1)  # over the ceiling while readable
        (f.dir / "pkg" / "a0_test.go").unlink()
        os.symlink("nowhere", f.dir / "pkg" / "a0_test.go")
        r = f.run("--worktree")
        self.assertEqual(r.returncode, 2, r.stdout + r.stderr)
        self.assertIn("could not be read", r.stderr)
        self.assertIn("a0_test.go", r.stderr)

    def test_a_zero_count_against_a_live_ceiling_refuses(self):
        """A corpus that held hundreds cannot drop to nothing without a commit that
        says so. A reader that stopped reading looks exactly like a drained backlog."""
        f = Fixture(tokens_go=0, ceiling=750)
        r = f.run()
        self.assertEqual(r.returncode, 2, r.stdout + r.stderr)
        self.assertIn("ZERO", r.stderr)

    def test_a_staged_new_test_file_does_not_red_the_gate(self):
        """The population must come from the same place as the content.

        Listing the INDEX and reading HEAD made every staged-but-uncommitted new
        test file 'unreadable', so the gate exited 2 for a session that had merely
        run `git add` -- and in a shared checkout that is somebody else's `git add`,
        on a run whose author touched nothing."""
        f = Fixture(tokens_go=1, ceiling=1)
        f.write("pkg/staged_test.go", TOKEN_GO.format("staged, not committed"))
        f._git("add", "pkg/staged_test.go")
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("1 token(s)", r.stdout)

    def test_a_zero_ceiling_with_zero_tokens_is_a_real_pass(self):
        """The drain finishing must not be indistinguishable from the reader breaking."""
        r = Fixture(tokens_go=0, ceiling=0).run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)


class TestTheCeilingItselfRatchets(unittest.TestCase):
    """Nothing else watches the ceiling. The design rests on "a reviewer sees the
    line", which is the property the repo's other ratchets refuse to rely on."""

    def test_a_raise_without_a_reason_fails(self):
        f = Fixture(tokens_go=1, ceiling=1)
        f.write("test/relax-ceiling.txt", "# prose\n9\n")
        r = f.run()
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("RAISED", r.stderr)

    def test_a_raise_with_a_reason_passes(self):
        f = Fixture(tokens_go=1, ceiling=1)
        f.write(
            "test/relax-ceiling.txt",
            "# prose\n# raised-for: TestFoo lost its only assertion with narrowTS\n9\n",
        )
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_an_old_reason_does_not_justify_a_later_raise(self):
        """The ratchet must not destroy itself on its first legitimate use.

        Asking only whether the marker is PRESENT meant one honest raise licensed
        every future one, and the ceiling file's own instructions tell raisers to
        add that line."""
        f = Fixture(tokens_go=1, ceiling=3)
        f.write(
            "test/relax-ceiling.txt",
            "# raised-for: an OLD raise, months ago\n3\n",
        )
        f.commit()
        f.write(
            "test/relax-ceiling.txt",
            "# raised-for: an OLD raise, months ago\n900\n",
        )
        r = f.run()
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("RAISED", r.stderr)

    def test_duplicating_an_old_reason_does_not_justify_a_raise(self):
        """A copy-paste is not a reason. Counted as a multiset, writing an existing
        `raised-for:` line a second time was a difference and bought any raise."""
        f = Fixture(tokens_go=1, ceiling=3)
        f.write("test/relax-ceiling.txt", "# raised-for: an OLD raise, months ago\n3\n")
        f.commit()
        f.write(
            "test/relax-ceiling.txt",
            "# raised-for: an OLD raise, months ago\n"
            "# raised-for: an OLD raise, months ago\n900\n",
        )
        r = f.run()
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("RAISED", r.stderr)

    def test_a_fresh_reason_beside_an_old_one_justifies_the_raise(self):
        f = Fixture(tokens_go=1, ceiling=3)
        f.write("test/relax-ceiling.txt", "# raised-for: an OLD raise\n3\n")
        f.commit()
        f.write(
            "test/relax-ceiling.txt",
            "# raised-for: an OLD raise\n# raised-for: TestBar lost narrowTS\n9\n",
        )
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_a_cosmetic_edit_to_an_old_reason_does_not_justify_a_raise(self):
        """The census keys `raised-for:` by the hook's `_reason_key`, so recasing a
        letter, adding a full stop or inserting a zero-width space is not a reason.
        Two halves of one gate disagreeing about sameness is how one becomes the way
        through."""
        for label, edited in (
            ("recased", "# raised-for: An OLD raise, months ago"),
            ("full stop", "# raised-for: an OLD raise, months ago."),
            ("zero width", "# raised-for: an OLD raise, months ago\u200b"),
            ("no space", "#raised-for: an OLD raise, months ago"),
        ):
            with self.subTest(label):
                f = Fixture(tokens_go=1, ceiling=3)
                f.write(
                    "test/relax-ceiling.txt",
                    "# raised-for: an OLD raise, months ago\n3\n",
                )
                f.commit()
                f.write("test/relax-ceiling.txt", edited + "\n900\n")
                r = f.run()
                self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
                self.assertIn("RAISED", r.stderr)

    def test_lower_cannot_launder_a_raise(self):
        """`--lower` measures against HEAD. Measured against the local file, a
        hand-raised ceiling could be rewritten to a value still above HEAD's and
        reported as a lowering."""
        f = Fixture(tokens_go=10, ceiling=5)
        f.write("test/relax-ceiling.txt", "# prose\n50\n")
        r = f.run("--lower")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("ABOVE", r.stderr)
        self.assertIn("\n50\n", (f.dir / "test" / "relax-ceiling.txt").read_text())

    def test_lowering_needs_no_reason(self):
        f = Fixture(tokens_go=1, ceiling=9)
        f.write("test/relax-ceiling.txt", "# prose\n2\n")
        r = f.run()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)


class TestSelftest(unittest.TestCase):
    def test_selftest_passes(self):
        r = subprocess.run(
            [sys.executable, str(SCRIPT), "--selftest"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("SELFTEST PASS", r.stdout)


if __name__ == "__main__":
    unittest.main(verbosity=2)
