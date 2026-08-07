#!/usr/bin/env python3
"""Unit tests for audit-test-relaxation.py, the test-relaxation gate.

(Named audit_relaxation_test.py rather than audit_test_relaxation_test.py: the
c_throwaway_tests hook check blocks a `_test_` infix outside internal/, test/ and
cmd/, which is a false positive here. Picked up by the *_test.py glob in
python_tests_test.go either way.)

Driven end-to-end through the real entry point (subprocess) rather than by
calling run_audit() directly, per the test corollary in
ai/rules/evidence.md: a unit test on a guard helper proves the helper
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
HOOK_LIB = REPO_ROOT / ".claude" / "hooks" / "lib"


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
        # The hook resolves its sibling lib/ relative to its OWN file, and
        # imports lib/session_id.py at module scope, so the fixture must give it
        # that neighbour or `exec_module` raises FileNotFoundError, load_detector
        # returns None, and every test here fails with "could not load detection
        # logic" instead of exercising the audit.
        os.symlink(HOOK_LIB, p / ".claude" / "hooks" / "lib")

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


class TestRelaxTokenInTheCarrierSyntax(unittest.TestCase):
    """A relaxation written in the file's own comment syntax must be REPORTED.

    `.ci` and `.et` scenarios comment with `#`, and this audit read `//` only, so a
    reason written the natural way was invisible: the finding downgraded from RELAXED,
    which prints the author's justification for the user to confirm, to WEAKENED, which
    prints no reason at all. The form that must never stop working is the Go one, since
    315 `.ci` files already carry `# // test-relax:`.

    End-to-end through the entry point, per this file's contract. The three helper
    assertions below it pin regex properties the exit code cannot isolate.
    """

    CI = "cmd=ze show\nexpect=out:text=one\nexpect=out:text=two\n"

    def _ci_fixture(self, tmp, body):
        fx = AuditFixture(tmp)
        Path(tmp, "test", "ui").mkdir(parents=True)
        Path(tmp, "test", "ui", "relax.ci").write_text(body)
        fx._commit("add the .ci")
        return fx

    def test_a_hash_reason_is_reported_as_a_relaxation(self):
        # VALIDATES: the natural `# test-relax:` reaches the verdict AND its text is
        # printed for the user to confirm.
        # PREVENTS: silently downgrading a justified .ci reduction to WEAKENED, which
        # tells the reviewer a test was weakened and nothing about why.
        with tempfile.TemporaryDirectory() as tmp:
            fx = self._ci_fixture(tmp, self.CI)
            Path(tmp, "test", "ui", "relax.ci").write_text(
                "# test-relax: the `two` command was removed\ncmd=ze show\n"
                "expect=out:text=one\n"
            )
            code, out = fx.audit()
            self.assertEqual(code, 1, out)
            self.assertIn("RELAXED", out)
            self.assertIn("the `two` command was removed", out)

    def test_the_reason_reported_is_the_one_that_was_added(self):
        # VALIDATES: reasons come back in FILE order, so run_audit's positional slice
        # names the new token.
        # PREVENTS: the regression a two-pattern union caused here -- Go matches were
        # concatenated ahead of hash matches, so a file already carrying a hash reason
        # that gained a Go one reported the OLD text as the addition.
        with tempfile.TemporaryDirectory() as tmp:
            fx = self._ci_fixture(tmp, "# test-relax: OLD reason\n" + self.CI)
            Path(tmp, "test", "ui", "relax.ci").write_text(
                "# test-relax: OLD reason\n# // test-relax: NEW reason\n"
                "cmd=ze show\nexpect=out:text=one\n"
            )
            code, out = fx.audit()
            self.assertEqual(code, 1, out)
            self.assertIn("NEW reason", out)
            self.assertNotIn("reason: OLD reason", out)

    def test_a_go_test_still_ignores_the_hash_form(self):
        # VALIDATES: `#` is not a Go comment, so a token written that way is no record
        # in the file's own syntax and must not silence the audit.
        # PREVENTS: a one-character escape from the weakening gate on every Go test.
        with tempfile.TemporaryDirectory() as tmp:
            fx = AuditFixture(tmp)
            Path(tmp, "pkg", "x_test.go").write_text(
                "package a\n# test-relax: nope\nfunc TestA(t *testing.T){}\n"
            )
            code, out = fx.audit()
            self.assertEqual(code, 1, out)
            self.assertIn("WEAKENED", out)
            self.assertNotIn("RELAXED", out)


class TestRelaxReasonRegexProperties(unittest.TestCase):
    """The two properties an exit code cannot isolate, asserted on the helper."""

    def setUp(self):
        self.mod = _load_audit_module()

    def test_one_line_is_never_counted_twice(self):
        # `# // test-relax:` must match at the `//` only: the `#` branch requires the
        # token immediately after it. A double count reads as an ADDED relaxation and
        # prints a phantom finding on a file nobody touched that way.
        self.assertEqual(
            self.mod.relax_reasons("# // test-relax: once\n", "a/b.ci"), ["once"]
        )

    def test_indentation_and_tabs_do_not_defeat_the_match(self):
        self.assertEqual(
            self.mod.relax_reasons("\t #\ttest-relax:\tspaced out\n", "a/b.ci"),
            ["spaced out"],
        )


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
