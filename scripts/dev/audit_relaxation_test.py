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

import importlib.util
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


def _load_audit_module():
    """Import audit-test-relaxation.py by path (its hyphenated name is not importable).

    Lets a test call the audit's own loader/detector functions directly, hermetically,
    without spinning up a git fixture.
    """
    spec = importlib.util.spec_from_file_location("ze_audit_relax", SCRIPT)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


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


class TestRfcDetectorIsShared(unittest.TestCase):
    """AC-17: the RFC detector is IMPORTED from the hook, never re-defined here."""

    def test_shared_detector_imported(self):
        # VALIDATES (AC-17): the audit reuses the hook's _rfc_tagged_change_err
        # rather than keeping its own copy, so the hook and the branch audit can
        # never drift apart on what counts as an RFC-tagged change.
        # PREVENTS: someone reimplementing the detector (or its _RFC_TAG /
        # _RFC_APPROVED regexes) locally in audit-test-relaxation.py, which would
        # let the two gates diverge silently.
        src = SCRIPT.read_text()
        self.assertNotIn(
            "def _rfc_tagged_change_err",
            src,
            "audit must IMPORT the RFC detector from the hook, not define its own copy",
        )
        self.assertNotIn(
            "_RFC_TAG",
            src,
            "the RFC-tag regex must live only in the hook, not be copied into the audit",
        )
        self.assertNotIn(
            "_RFC_APPROVED",
            src,
            "the approval-token regex must live only in the hook, not be copied here",
        )
        # And prove the detector the audit actually loads ORIGINATES in the hook
        # file. co_filename is the discriminator: a local reimplementation would
        # report audit-test-relaxation.py here and fail this assertion.
        mod = _load_audit_module()
        detector = mod.load_rfc_detector(REPO_ROOT)
        self.assertIsNotNone(
            detector, "audit failed to import the RFC detector from the hook"
        )
        self.assertEqual(detector.__name__, "_rfc_tagged_change_err")
        self.assertTrue(
            detector.__code__.co_filename.endswith("pretool-writeedit.py"),
            f"detector must be defined in the hook, got "
            f"{detector.__code__.co_filename!r}",
        )


class TestRfcTaggedChangeSurfaced(unittest.TestCase):
    """AC-18: a branch diff that changes an RFC-tagged test is surfaced by the audit."""

    RFC_OLD = (
        "package a\n"
        "// RFC requirement: RFC4271-9\n"
        "func TestHoldtime(t *testing.T){ require.Equal(t, 30, holdtime()) }\n"
    )
    # Same test, expected value swapped 30 -> 90, NO approval token: a behavior
    # change the count-based weakening heuristic cannot see (every count is equal).
    RFC_NEW = (
        "package a\n"
        "// RFC requirement: RFC4271-9\n"
        "func TestHoldtime(t *testing.T){ require.Equal(t, 90, holdtime()) }\n"
    )
    # The same value swap, but now carrying a self-written approval token.
    RFC_NEW_APPROVED = (
        "package a\n"
        "// RFC requirement: RFC4271-9\n"
        "// rfc-test-change-approved: 2026-07-17 widen holdtime per user\n"
        "func TestHoldtime(t *testing.T){ require.Equal(t, 90, holdtime()) }\n"
    )

    def test_branch_diff_surfaces_rfc_test_change(self):
        # VALIDATES (AC-18): an RFC-tagged test whose expected value changes with
        # NO approval token is reported by the audit's RFC path. Swapping 30 -> 90
        # keeps every weakening count identical, so only the RFC detector catches it.
        # PREVENTS: an equal-count edit to a compliance test passing the audit silently.
        #
        # The NEGATIVE half documents the real, honest behavior from Fix 3: the SAME
        # shared detector returns None once a self-written rfc-test-change-approved:
        # token is present, so the branch audit is silenced exactly like the hook.
        # PREVENTS: re-introducing the false claim that the audit is a backstop against
        # a token an agent wrote for itself (the only real backstop is grep + review).
        mod = _load_audit_module()
        rfc_detector = mod.load_rfc_detector(REPO_ROOT)
        self.assertIsNotNone(
            rfc_detector, "audit failed to import the RFC detector from the hook"
        )
        tags = rfc_detector(self.RFC_OLD, self.RFC_NEW, "pkg/holdtime_test.go")
        self.assertTrue(
            tags, "unapproved RFC-tagged behavior change must be surfaced by the audit"
        )
        self.assertTrue(
            any("RFC4271-9" in t for t in tags),
            f"the reported tag must name the RFC requirement, got {tags!r}",
        )
        approved = rfc_detector(
            self.RFC_OLD, self.RFC_NEW_APPROVED, "pkg/holdtime_test.go"
        )
        self.assertIsNone(
            approved,
            "a forged rfc-test-change-approved: token suppresses BOTH gates (Fix 3)",
        )


if __name__ == "__main__":
    unittest.main()
