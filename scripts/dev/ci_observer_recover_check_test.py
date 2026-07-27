#!/usr/bin/env python3
"""Unit tests for ci_observer_recover_check.

Collected by TestPythonUnitTests (scripts/dev/python_tests_test.go).

The last test class is the ratchet itself: it runs the real scan over the real
test tree and asserts zero findings. That is what makes `make ze-unit-test`
enforce the invariant, rather than leaving it to a make target nobody runs.
"""

import unittest
from pathlib import Path

import ci_observer_recover_check as chk


class TestEngineReachingNames(unittest.TestCase):
    """The flagged call set is DERIVED from ze_api, never hand-listed."""

    def test_transitive_closure_from_the_guard_call_sites(self):
        source = """
def _call_engine(self, method, params=None):
    pass

def wait_for_shutdown(self, timeout=5.0):
    pass

def dispatch(api, command):
    return api._call_engine("x", {})

def dispatch_until(api, command, predicate):
    return dispatch(api, command)

def result_text_data(result, default=""):
    return default
"""
        names = chk.engine_reaching_names(source)

        self.assertIn("_call_engine", names)
        self.assertIn("wait_for_shutdown", names)
        self.assertIn("dispatch", names, "direct caller of _call_engine")
        self.assertIn("dispatch_until", names, "indirect caller, two hops out")
        self.assertNotIn("result_text_data", names, "a pure helper must not be flagged")

    def test_real_ze_api_yields_the_expected_shape(self):
        """Guard against the derivation silently collapsing to nothing."""
        root = chk.repo_root()
        names = chk.engine_reaching_names((root / "test/scripts/ze_api.py").read_text())

        # If this ever drops to just the two entry points, the closure broke and
        # the gate would pass vacuously.
        self.assertGreater(
            len(names), 5, f"suspiciously small closure: {sorted(names)}"
        )
        for expected in (
            "dispatch",
            "dispatch_until",
            "_call_engine",
            "wait_for_shutdown",
        ):
            self.assertIn(expected, names)


class TestExtractPythonBlocks(unittest.TestCase):
    """tmpfs parsing must match internal/test/tmpfs/tmpfs.go."""

    def test_extracts_py_and_run_blocks_only(self):
        ci = "\n".join(
            [
                "cmd=background",
                "tmpfs=peer.conf:terminator=EOF_CONF",
                "peer 127.0.0.1 { local-as 65533 }",
                "EOF_CONF",
                "tmpfs=observer.py:terminator=EOF_PY",
                "import ze_api",
                "EOF_PY",
                "tmpfs=plugin.run:terminator=EOF_RUN",
                "print('hi')",
                "EOF_RUN",
            ]
        )
        blocks = chk.extract_python_blocks(ci)

        self.assertEqual([b[0] for b in blocks], ["observer.py", "plugin.run"])
        self.assertEqual(blocks[0][2], "import ze_api")

    def test_line_offsets_point_back_at_the_ci_file(self):
        ci = "\n".join(
            [
                "cmd=background",  # 1
                "tmpfs=observer.py:terminator=EOF_PY",  # 2
                "import ze_api",  # 3  <- first content line
                "EOF_PY",
            ]
        )
        blocks = chk.extract_python_blocks(ci)
        self.assertEqual(blocks[0][1], 3)

    def test_base64_blocks_are_skipped(self):
        ci = "\n".join(
            [
                "tmpfs=blob.py:encoding=base64:terminator=EOF_B64",
                "aW1wb3J0IHplX2FwaQ==",
                "EOF_B64",
            ]
        )
        self.assertEqual(chk.extract_python_blocks(ci), [])

    def test_header_without_terminator_is_ignored(self):
        self.assertEqual(chk.extract_python_blocks("tmpfs=observer.py\nbody\n"), [])


ENGINE_NAMES = {"dispatch", "_call_engine", "wait_for_shutdown", "send"}


class TestScanBlock(unittest.TestCase):
    """Polarity: flag recovering handlers, leave failure paths alone."""

    def test_flags_engine_call_in_a_recovering_handler(self):
        source = """
for cmd in commands:
    try:
        api.send(cmd)
    except RuntimeError:
        dispatch(api, 'show bgp summary')
        continue
"""
        findings = chk.scan_block(source, ENGINE_NAMES)
        self.assertEqual([name for _, name in findings], ["dispatch"])

    def test_ignores_a_handler_that_reraises(self):
        source = """
try:
    api.send('x')
except RuntimeError:
    dispatch(api, 'request shutdown')
    raise
"""
        self.assertEqual(chk.scan_block(source, ENGINE_NAMES), [])

    def test_ignores_a_handler_that_exits(self):
        """The real test/plugin/cursor-replay.ci shape: firing IS correct there."""
        source = """
import sys
try:
    api.send('x')
except RuntimeError as e:
    print(f'FAIL: {e}', file=sys.stderr)
    dispatch(api, 'request shutdown')
    api.wait_for_shutdown()
    sys.exit(1)
"""
        self.assertEqual(chk.scan_block(source, ENGINE_NAMES), [])

    def test_ignores_a_handler_that_calls_runtime_fail(self):
        source = """
try:
    api.send('x')
except RuntimeError:
    dispatch(api, 'request shutdown')
    runtime_fail('send rejected')
"""
        self.assertEqual(chk.scan_block(source, ENGINE_NAMES), [])

    def test_ignores_non_engine_calls(self):
        source = """
try:
    api.send('x')
except RuntimeError:
    print('handled')
    time.sleep(0.1)
"""
        self.assertEqual(chk.scan_block(source, ENGINE_NAMES), [])

    def test_ignores_engine_calls_outside_any_handler(self):
        source = """
dispatch(api, 'show bgp summary')
try:
    api.send('x')
except RuntimeError:
    pass
"""
        self.assertEqual(chk.scan_block(source, ENGINE_NAMES), [])

    def test_exempt_marker_on_the_same_line(self):
        source = """
try:
    api.send('x')
except RuntimeError:
    dispatch(api, 'request shutdown')  # ze-observer-check: deliberate failure path
    continue
"""
        self.assertEqual(chk.scan_block(source, ENGINE_NAMES), [])

    def test_exempt_marker_on_the_line_above(self):
        source = """
try:
    api.send('x')
except RuntimeError:
    # ze-observer-check: deliberate failure path
    dispatch(api, 'request shutdown')
    continue
"""
        self.assertEqual(chk.scan_block(source, ENGINE_NAMES), [])

    def test_non_python_block_is_silently_skipped(self):
        self.assertEqual(chk.scan_block("#!/bin/sh\nexit 0\n", ENGINE_NAMES), [])


class TestRepoRatchet(unittest.TestCase):
    """THE ratchet: the guard's no-false-positive claim, checked every run."""

    def test_no_engine_call_in_a_recovering_handler_anywhere(self):
        root = chk.repo_root()
        engine_names = chk.engine_reaching_names(
            (root / "test/scripts/ze_api.py").read_text()
        )

        offenders = []
        blocks = 0
        for ci in sorted((root / "test").rglob("*.ci")):
            try:
                text = ci.read_text()
            except (OSError, UnicodeDecodeError):
                continue
            for name, offset, source in chk.extract_python_blocks(text):
                blocks += 1
                for lineno, called in chk.scan_block(source, engine_names):
                    offenders.append(
                        f"{ci.relative_to(root)}:{offset + lineno - 1} "
                        f"({name}) calls {called}()"
                    )

        # A vacuous pass here would be indistinguishable from a clean one.
        self.assertGreater(blocks, 100, "observer blocks stopped being found")
        self.assertEqual(
            offenders,
            [],
            "engine call inside a recovering except handler; ze_api's guard "
            "would fire the ZE-OBSERVER-FAIL sentinel and fail these tests "
            "spuriously:\n  " + "\n  ".join(offenders),
        )


if __name__ == "__main__":
    unittest.main()
