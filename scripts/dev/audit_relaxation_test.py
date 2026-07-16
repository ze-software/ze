#!/usr/bin/env python3
"""Unit tests for audit-test-relaxation.py, the test-relaxation gate.

(Named audit_relaxation_test.py rather than audit_test_relaxation_test.py: the
c_throwaway_tests hook check blocks a `_test_` infix outside internal/, test/ and
cmd/, which is a false positive here. Picked up by the *_test.py glob in
python_tests_test.go either way.)

Driven end-to-end through the real entry point (subprocess) rather than by
calling run_audit() directly, per the test corollary in
ai/rules/fail-closed-guards.md: a unit test on a guard helper proves the helper
works, not that the caller reaches it with the input that matters. The exit code
IS the gate, so the gate is what gets asserted.

Each fixture repo symlinks the real .claude/hooks/pretool-writeedit.py into
place, because main() loads the weakening detector from there and exits 2 when it
cannot. Without the symlink every test would pass on a detector-load failure
instead of on the behaviour it names.

The central property: "clean" must mean "I compared things and found nothing",
never "I compared nothing".
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "audit-test-relaxation.py"
REPO_ROOT = HERE.parent.parent
HOOK = REPO_ROOT / ".claude" / "hooks" / "pretool-writeedit.py"

HEALTHY = (
    "package a\n"
    "func TestA(t *testing.T){ require.Equal(t,1,f()); require.NoError(t,err) }\n"
)
# Adds t.Skip and drops an assertion: two independent weakening signals.
WEAKENED = (
    'package a\nfunc TestA(t *testing.T){ t.Skip("x"); require.Equal(t,1,f()) }\n'
)


def _git(repo, *args, env=None):
    return subprocess.run(
        ["git", *args], cwd=repo, capture_output=True, text=True, env=env
    )


class AuditFixture:
    """A throwaway git repo whose main branch CONTAINS a committed weakening.

    Mirrors the ze repo's actual workflow: commits land directly on main, so
    `main` and HEAD are the same commit and the weakening is only visible
    against an earlier base (the stand-in for origin/main).
    """

    def __init__(self, tmp):
        self.repo = tmp
        self.env = dict(os.environ, GIT_EDITOR="true")
        p = Path(tmp)
        (p / ".claude" / "hooks").mkdir(parents=True)
        # Symlink, not copy: the hook is 67K and the audit only imports it.
        os.symlink(HOOK, p / ".claude" / "hooks" / "pretool-writeedit.py")

        _git(tmp, "init", "-q", "-b", "main", env=self.env)
        _git(tmp, "config", "user.email", "t@example.com", env=self.env)
        _git(tmp, "config", "user.name", "Test", env=self.env)
        # Throwaway fixture repo: never our own commits, so signing is off.
        _git(tmp, "config", "commit.gpgsign", "false", env=self.env)

        (p / "pkg").mkdir()
        (p / "pkg" / "x_test.go").write_text(HEALTHY)
        self._commit("baseline")
        self.base_sha = _git(tmp, "rev-parse", "HEAD", env=self.env).stdout.strip()

        # The weakening is COMMITTED on main, exactly as it would be here.
        (p / "pkg" / "x_test.go").write_text(WEAKENED)
        self._commit("weaken TestA")

    def _commit(self, msg):
        _git(self.repo, "add", "-A", env=self.env)
        _git(self.repo, "commit", "-q", "-m", msg, env=self.env)

    def audit(self, *args):
        """Run the real script from inside the fixture; return (code, output)."""
        r = subprocess.run(
            [sys.executable, str(SCRIPT), *args],
            cwd=self.repo,
            capture_output=True,
            text=True,
            env=dict(self.env, NO_COLOR="1"),
        )
        return r.returncode, r.stdout + r.stderr


class TestEmptyRangeIsNeverClean(unittest.TestCase):
    """The defect: an audit that compared nothing reported a clean bill of health."""

    def test_base_equal_to_head_is_not_reported_clean(self):
        # VALIDATES: `audit main` while ON main (this repo's normal state) must
        # not print a clean verdict; main..HEAD is empty, so nothing was audited.
        # PREVENTS: the documented invocation silently degrading to a worktree
        # diff and reporting "clean" over a branch full of weakened tests.
        with tempfile.TemporaryDirectory() as tmp:
            fx = AuditFixture(tmp)
            code, out = fx.audit("main")
            self.assertNotIn(
                "clean",
                out.lower(),
                f"audited an empty range and called it clean: {out!r}",
            )
            self.assertNotEqual(
                code, 0, f"empty range must not pass the gate; got exit 0:\n{out}"
            )
            # Says something actionable rather than merely failing.
            self.assertIn("main", out, f"refusal should name the base: {out!r}")

    def test_nonexistent_base_is_not_reported_clean(self):
        # VALIDATES: a base that does not resolve is refused, not reported clean.
        # PREVENTS: a typo'd base ref ("orgin/main") silently passing the gate,
        # which the `git diff` failure path turned into an empty finding list.
        with tempfile.TemporaryDirectory() as tmp:
            fx = AuditFixture(tmp)
            code, out = fx.audit("no-such-ref-xyz")
            self.assertNotIn(
                "clean", out.lower(), f"unresolvable base called clean: {out!r}"
            )
            self.assertEqual(
                code, 2, f"unusable base should exit 2 (audit could not run):\n{out}"
            )


class TestRealComparisonsStillWork(unittest.TestCase):
    """The guard must not buy fail-closed by breaking the valid invocations."""

    def test_committed_weakening_is_reported_against_an_earlier_base(self):
        # VALIDATES: the honest invocation (a base that is a real ancestor, the
        # stand-in for origin/main) still reports the committed weakening.
        # PREVENTS: "fixing" the false green by making the tool report nothing.
        with tempfile.TemporaryDirectory() as tmp:
            fx = AuditFixture(tmp)
            code, out = fx.audit(fx.base_sha)
            self.assertEqual(code, 1, f"expected findings (exit 1):\n{out}")
            self.assertIn("WEAKENED", out)
            self.assertIn("pkg/x_test.go", out)

    def test_default_base_audits_uncommitted_changes(self):
        # VALIDATES: the default (no argument) still diffs the worktree against
        # HEAD. anchor == HEAD is legitimate here and must not trip the guard.
        # PREVENTS: the empty-range guard swallowing the tool's default mode.
        with tempfile.TemporaryDirectory() as tmp:
            fx = AuditFixture(tmp)
            # Weaken further, uncommitted: drop the remaining assertion.
            Path(tmp, "pkg", "x_test.go").write_text(
                'package a\nfunc TestA(t *testing.T){ t.Skip("x") }\n'
            )
            code, out = fx.audit()
            self.assertEqual(code, 1, f"expected findings (exit 1):\n{out}")
            self.assertIn("pkg/x_test.go", out)

    def test_clean_verdict_is_still_reachable_when_nothing_changed(self):
        # VALIDATES: a real comparison that finds nothing still reports clean.
        # PREVENTS: over-correcting into a gate that can never go green, which
        # would train reviewers to ignore it.
        with tempfile.TemporaryDirectory() as tmp:
            fx = AuditFixture(tmp)
            code, out = fx.audit()  # HEAD vs an untouched worktree
            self.assertEqual(code, 0, f"expected a clean pass:\n{out}")
            self.assertIn("clean", out.lower())


class TestSelftest(unittest.TestCase):
    def test_selftest_passes(self):
        # VALIDATES: the script's own --selftest still passes after the change.
        # PREVENTS: refactoring run_audit's signature out from under it.
        with tempfile.TemporaryDirectory() as tmp:
            fx = AuditFixture(tmp)
            code, out = fx.audit("--selftest")
            self.assertEqual(code, 0, out)
            self.assertIn("SELFTEST PASS", out)


if __name__ == "__main__":
    unittest.main()
