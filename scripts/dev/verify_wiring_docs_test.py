#!/usr/bin/env python3
"""Unit tests for the composable ci-sleep DELTA ratchet in verify_wiring_docs.py.

The ratchet used to store one absolute integer in test/.ci-sleep-baseline, so
two specs that each lowered it collided on the second merge (spec-fixit-sleeps-
cli-harness 132->129 vs spec-fixit-reject-fence-observability 132->130). The
delta form stores a column of signed integers that SUM to the ceiling: parallel
removals append distinct `-N` lines and never touch a shared integer, yet a net
rise still fails. This test pins that arithmetic and the monotonic guarantee.
"""

from __future__ import annotations

import io
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from verify_wiring_docs import check_ci_sleep_ratchet, parse_sleep_baseline


def write(root: Path, rel: str, body: str) -> None:
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(body, encoding="utf-8")


def ci_with_sleeps(n: int) -> str:
    return "".join(f"time.sleep(0.1)  # deliberate\n" for _ in range(n))


class ParseSleepBaselineTest(unittest.TestCase):
    def test_plain_int_backward_compatible(self):
        # A pre-existing single-integer baseline still parses to that ceiling.
        self.assertEqual(parse_sleep_baseline("125\n"), 125)

    def test_signed_int_lines_sum(self):
        # Origin plus two independent removal deltas.
        self.assertEqual(parse_sleep_baseline("125\n-1\n-1\n"), 123)

    def test_comments_and_blanks_ignored(self):
        text = "# header\n\n125\n# a removal\n-3\n"
        self.assertEqual(parse_sleep_baseline(text), 122)

    def test_positive_delta_raises_ceiling(self):
        # The explicit-approval knob: a `+N` line is how the ceiling is raised
        # (equivalent to editing the old absolute integer upward).
        self.assertEqual(parse_sleep_baseline("125\n+2\n"), 127)

    def test_no_integer_lines_is_inactive(self):
        self.assertIsNone(parse_sleep_baseline("# only comments\n"))


class SleepRatchetDeltaTest(unittest.TestCase):
    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="sleep-ratchet-")
        self.addCleanup(lambda: __import__("shutil").rmtree(d, ignore_errors=True))
        return Path(d)

    def _run(self, root: Path) -> tuple[int, str]:
        buf = io.StringIO()
        with redirect_stdout(buf):
            rc = check_ci_sleep_ratchet(root, ["test/a.ci"])
        return rc, buf.getvalue()

    def test_sleep_ratchet_delta_composes(self):
        # Two independent removals recorded as separate `-1` lines both take
        # effect (the parser sums them): ceiling 4 - 1 - 1 = 2. Tree now holds 2
        # sleeps -> at the ceiling, passes.
        root = self._root()
        write(root, "test/a.ci", ci_with_sleeps(1))
        write(root, "test/b.ci", ci_with_sleeps(1))
        write(root, "test/.ci-sleep-baseline", "4\n-1\n-1\n")
        rc, out = self._run(root)
        self.assertEqual(rc, 0, out)

    def test_net_rise_still_fails(self):
        # Same delta ceiling (2), but the tree now holds 3 sleeps: a net rise
        # must fail even though the removals were recorded as deltas.
        root = self._root()
        write(root, "test/a.ci", ci_with_sleeps(2))
        write(root, "test/b.ci", ci_with_sleeps(1))
        write(root, "test/.ci-sleep-baseline", "4\n-1\n-1\n")
        rc, out = self._run(root)
        self.assertEqual(rc, 1, out)
        self.assertIn("ratchet FAILED", out)

    def test_net_zero_boundary_passes(self):
        # count == ceiling is the last valid value (boundary).
        root = self._root()
        write(root, "test/a.ci", ci_with_sleeps(2))
        write(root, "test/.ci-sleep-baseline", "2\n")
        rc, out = self._run(root)
        self.assertEqual(rc, 0, out)

    def test_under_ceiling_is_advisory_not_failure(self):
        # Fewer sleeps than the ceiling is fine (ratchet only fails on a rise);
        # it prints an advisory to tighten the baseline.
        root = self._root()
        write(root, "test/a.ci", ci_with_sleeps(1))
        write(root, "test/.ci-sleep-baseline", "4\n")
        rc, out = self._run(root)
        self.assertEqual(rc, 0, out)


if __name__ == "__main__":
    unittest.main()
