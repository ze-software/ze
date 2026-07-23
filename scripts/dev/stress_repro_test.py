#!/usr/bin/env python3
"""Unit tests for scripts/dev/stress-repro.py.

Picked up automatically by TestPythonUnitTests (scripts/dev/python_tests_test.go),
which globs scripts/dev/*_test.py -- see ai/rules/testing.md "Testing Python
Tooling".

These exist because the reproducer had three defects that nothing could catch:
  * the TimeoutExpired path did `bytes + str` and raised TypeError, so the tool
    crashed on exactly the failure it exists to find;
  * a usage error was reported as `*** REPRODUCED ***`, recording an operator
    typo as a product failure;
  * the fix for that was written as a helper and never wired to its call site,
    so the guarded behaviour was dead code while the bug stayed live.

The last one is why `test_usage_helper_is_actually_wired` is here: a helper that
nothing calls passes every test written about the helper.
"""

import importlib.util
import pathlib
import subprocess
import unittest

HERE = pathlib.Path(__file__).resolve().parent
SCRIPT = HERE / "stress-repro.py"


def load():
    spec = importlib.util.spec_from_file_location("stress_repro", SCRIPT)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class TestAsText(unittest.TestCase):
    def test_handles_every_shape_subprocess_produces(self):
        m = load()
        self.assertEqual(m._as_text(None), "")
        self.assertEqual(m._as_text("already str"), "already str")
        self.assertEqual(m._as_text(b"raw bytes"), "raw bytes")
        self.assertEqual(m._as_text(bytearray(b"mutable")), "mutable")

    def test_undecodable_bytes_do_not_raise(self):
        m = load()
        self.assertIn("�", m._as_text(b"\xff\xfe not utf-8"))

    def test_timeout_path_does_not_concatenate_bytes_with_str(self):
        """TimeoutExpired carries UNDECODED streams even under text=True."""
        m = load()
        exc = subprocess.TimeoutExpired(cmd=["x"], timeout=1, output=b"partial")
        exc.stderr = b"err"
        combined = m._as_text(exc.stdout) + m._as_text(exc.stderr)
        self.assertEqual(combined, "partialerr")


class TestUsageErrorSignature(unittest.TestCase):
    def test_detects_a_real_usage_error(self):
        m = load()
        out = (
            "unknown command: reload\nze-test\n\nUsage:\n\nCommands:\n  bgp  Run BGP\n"
        )
        self.assertEqual(m.usage_error_signature(out), "unknown command:")

    def test_does_not_swallow_a_genuine_failure_that_quotes_the_phrase(self):
        """test/ui/root-namespace.ci asserts `unknown command: traffic-control`.

        When it genuinely fails the runner echoes both the unmet needle and ze's
        stderr, so the phrase appears in a REAL reproduction. Discarding that as
        a typo would throw away the capture the tool exists to produce.
        """
        m = load()
        out = (
            'stderr does not contain "unknown command: traffic-control"\n'
            "CLIENT OUTPUT:\nunknown command: traffic-control\n"
            "fail  1/40  2.5%\n"
        )
        self.assertIsNone(m.usage_error_signature(out))

    def test_ignores_output_with_no_signature(self):
        m = load()
        self.assertIsNone(m.usage_error_signature("pass  40/40  100.0%\n"))


class TestWiring(unittest.TestCase):
    def test_usage_helper_is_actually_wired(self):
        """A guard nothing calls is not a guard.

        The first version of this fix defined usage_error_signature and left the
        call site running the original unguarded expression, so the helper was
        dead code and the bug it was written for stayed live. Caught by review,
        not by any test -- hence this one.
        """
        body = SCRIPT.read_text(encoding="utf-8")
        calls = [
            line
            for line in body.splitlines()
            if "usage_error_signature(" in line
            and not line.lstrip().startswith("#")
            and not line.lstrip().startswith("def ")
        ]
        self.assertTrue(
            calls,
            "usage_error_signature is defined but never called: the guard is dead code",
        )

    def test_no_unguarded_usage_signature_scan_remains(self):
        """The original expression must be gone, not merely bypassed."""
        body = SCRIPT.read_text(encoding="utf-8")
        self.assertNotIn(
            "(s for s in USAGE_SIGNATURES if s in out)",
            body.replace(
                "    return next((s for s in USAGE_SIGNATURES if s in out), None)", ""
            ),
            "an unguarded USAGE_SIGNATURES scan is still present outside the helper",
        )


class TestFeatureGateTags(unittest.TestCase):
    def test_matches_the_manifest(self):
        m = load()
        tags = m._feature_gate_tags()
        self.assertIn("ze_bgp", tags)
        self.assertEqual(tags, sorted(set(tags)), "tags must be sorted and unique")
        self.assertTrue(all(t.startswith("ze_") for t in tags))


if __name__ == "__main__":
    unittest.main()
