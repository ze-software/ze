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
import os
import pathlib
import subprocess
import sys
import tempfile
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
        # Count occurrences rather than string-surgery on one exact source line:
        # the earlier form stripped a literal line and would false-positive on a
        # reformat. USAGE_SIGNATURES may be iterated exactly once, inside the
        # helper.
        body = SCRIPT.read_text(encoding="utf-8")
        scans = [
            line
            for line in body.splitlines()
            if "for s in USAGE_SIGNATURES" in line and not line.lstrip().startswith("#")
        ]
        self.assertEqual(
            len(scans),
            1,
            f"USAGE_SIGNATURES must be scanned only inside the helper, found: {scans}",
        )


class TestFeatureGateTags(unittest.TestCase):
    def test_matches_the_manifest(self):
        m = load()
        tags = m._feature_gate_tags()
        self.assertIn("ze_bgp", tags)
        self.assertEqual(tags, sorted(set(tags)), "tags must be sorted and unique")
        self.assertTrue(all(t.startswith("ze_") for t in tags))


class TestRunSlug(unittest.TestCase):
    """The log name must survive every selector ze-test accepts.

    ai/rules/testing.md tells you to select by NAME rather than by numeric id in
    anything you keep. A name carries `/`, and the old join put it straight into
    the log path: the tool died with FileNotFoundError before running anything,
    on the selector form the rules prefer.
    """

    def test_a_name_selector_stays_one_filename_component(self):
        m = load()
        slug = m.run_slug("web", "test/web/commit-flow.wb")
        self.assertNotIn("/", slug)
        self.assertIn("commit-flow.wb", slug)

    def test_a_subsuite_and_numeric_selector_are_unchanged(self):
        m = load()
        self.assertEqual(m.run_slug("bgp plugin", "97"), "bgp-plugin-97")

    def test_no_selector_still_names_the_suite(self):
        m = load()
        self.assertEqual(m.run_slug("web --draft", ""), "web-draft")
        self.assertEqual(m.run_slug("web", None), "web")

    def test_main_assigns_the_log_path_it_prints(self):
        """run_slug must be wired into main, not merely defined.

        The helper landed and its `logpath = ...` call site did not, so every
        invocation died with `NameError: name 'logpath' is not defined` on the
        header print, before a single test ran. The tests above all passed:
        they call run_slug directly, which is exactly the blind spot
        test_usage_helper_is_actually_wired was written for the first time this
        happened.
        """
        body = SCRIPT.read_text(encoding="utf-8")
        assigns = [
            line
            for line in body.splitlines()
            if line.lstrip().startswith("logpath =") and "run_slug(" in line
        ]
        self.assertEqual(
            len(assigns),
            1,
            f"main must assign logpath from run_slug exactly once, found: {assigns}",
        )


class TestMainRuns(unittest.TestCase):
    """main() must run end to end, not die on a name it never bound.

    Every test above imports the module and calls a helper, so a NameError in
    main is invisible to all of them -- and that is what shipped: the tool died
    on `logpath` before starting a single test. This drives the entry point,
    with two stub binaries standing in for ze and ze-test so the run reaches
    the loop, finishes, and reports. A healthy tool prints its log path and
    exits 1 (nothing reproduced). A broken one exits 1 too, which is why the
    traceback and the log line are both asserted.
    """

    def test_entry_point_runs_to_completion(self):
        with tempfile.TemporaryDirectory() as tmp:
            stubs = {}
            for name in ("ze", "ze-test"):
                path = pathlib.Path(tmp) / name
                path.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
                path.chmod(0o755)
                stubs[name] = str(path)

            env = dict(os.environ)
            env["ZE_BIN"] = stubs["ze"]
            env["ZE_TEST_BIN"] = stubs["ze-test"]
            proc = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "bgp plugin",
                    "--test",
                    "4",
                    "--iterations",
                    "1",
                    "--parallel",
                    "1",
                    "--burners",
                    "1",
                    "--minutes",
                    "1",
                ],
                capture_output=True,
                text=True,
                env=env,
                timeout=120,
            )

        output = proc.stdout + proc.stderr
        self.assertNotIn("Traceback", output, output)
        self.assertIn(
            "log:", proc.stdout, f"the run never printed its capture path\n{output}"
        )
        self.assertEqual(
            proc.returncode,
            1,
            f"want the not-reproduced exit; got {proc.returncode}\n{output}",
        )


if __name__ == "__main__":
    unittest.main()
