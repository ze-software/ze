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
from verify_wiring_docs import (
    MAKE_TARGETS,
    TARGET_ORDER,
    check_ci_sleep_ratchet,
    check_known_failure_load_excuses,
    parse_sleep_baseline,
    selected_targets,
)


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


class KnownFailureLoadExcuseTest(unittest.TestCase):
    """The gate behind ai/rules/fix-dont-record.md.

    A shard blaming host load is stating a diagnosis (the test asserts on elapsed
    time) and calling it a mystery. The gate rejects the excuse, NOT the shard: a
    red whose mechanism is genuinely unknown still belongs in the directory.
    """

    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="load-excuse-")
        self.addCleanup(lambda: __import__("shutil").rmtree(d, ignore_errors=True))
        return Path(d)

    def _run(self, root: Path, changed: list[str]) -> tuple[int, str]:
        buf = io.StringIO()
        with redirect_stdout(buf):
            rc = check_known_failure_load_excuses(root, changed)
        return rc, buf.getvalue()

    def test_load_excuse_fails_and_names_the_line(self):
        root = self._root()
        write(
            root,
            "plan/known-failures/flaky.md",
            "### suite N -- flaky\n\nIt only fails on a loaded host.\n",
        )
        rc, out = self._run(root, ["plan/known-failures/flaky.md"])
        self.assertEqual(rc, 1, out)
        self.assertIn("plan/known-failures/flaky.md:3", out)

    def test_every_banned_phrase_is_caught(self):
        # One shard per phrase, so a regex edit that drops an alternative fails
        # here rather than silently reopening that wording.
        phrases = [
            "it fails under load",
            "only on a loaded host",
            "load average was 11",
            "this test is load-sensitive",
            "all three pass in isolation",
            "it passed in isolation",
            "attributed to resource contention",
            "seen on a contended host",
        ]
        for i, phrase in enumerate(phrases):
            with self.subTest(phrase=phrase):
                root = self._root()
                rel = f"plan/known-failures/s{i}.md"
                write(root, rel, f"### red\n\n{phrase}.\n")
                rc, out = self._run(root, [rel])
                self.assertEqual(rc, 1, out)

    def test_unknown_mechanism_shard_still_allowed(self):
        # The directory is not closed: a shard that does not blame load passes.
        root = self._root()
        write(
            root,
            "plan/known-failures/mystery.md",
            "### suite N\n\nFails once in 200 runs; mechanism unknown.\n"
            "Repro: scripts/dev/stress-repro.py ...\nNext: read the producer.\n",
        )
        rc, out = self._run(root, ["plan/known-failures/mystery.md"])
        self.assertEqual(rc, 0, out)

    def test_readme_and_resolved_are_exempt(self):
        # README states the policy (so it must quote the phrases) and RESOLVED is
        # a verbatim archive that is never edited to satisfy a present-day gate.
        root = self._root()
        for name in ("README.md", "RESOLVED.md"):
            write(root, f"plan/known-failures/{name}", "fails under load\n")
        rc, out = self._run(
            root,
            ["plan/known-failures/README.md", "plan/known-failures/RESOLVED.md"],
        )
        self.assertEqual(rc, 0, out)

    def test_deleted_shard_is_not_a_violation(self):
        # Deleting a shard is the intended outcome of fixing the test, and a
        # deleted path still appears in the changed set.
        root = self._root()
        rc, out = self._run(root, ["plan/known-failures/gone.md"])
        self.assertEqual(rc, 0, out)

    def test_unrelated_changed_files_do_not_trigger(self):
        root = self._root()
        write(root, "docs/perf.md", "throughput degrades under load\n")
        rc, out = self._run(root, ["docs/perf.md"])
        self.assertEqual(rc, 0, out)
        self.assertEqual(out, "")


class LearnedStalenessRoutingTest(unittest.TestCase):
    """Changed-file routing for the learned-summary staleness gate.

    Wiring row: a changed plan/learned/**.md must select ze-learned-staleness,
    so `make ze-verify-changed` runs the gate exactly when a summary's cited
    paths or NNN citations could have gone dangling.
    """

    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="learned-routing-")
        self.addCleanup(lambda: __import__("shutil").rmtree(d, ignore_errors=True))
        return Path(d)

    def test_learned_change_selects_staleness_target(self):
        root = self._root()
        self.assertIn(
            "ze-learned-staleness",
            selected_targets(root, ["plan/learned/0999-example.md"]),
        )

    def test_checker_and_baseline_select_the_target(self):
        root = self._root()
        for path in (
            "scripts/dev/learned_staleness.py",
            "plan/.learned-staleness-baseline",
        ):
            with self.subTest(path=path):
                self.assertIn("ze-learned-staleness", selected_targets(root, [path]))

    def test_unrelated_change_does_not_select_it(self):
        root = self._root()
        self.assertNotIn(
            "ze-learned-staleness",
            selected_targets(root, ["docs/guide/monitoring.md"]),
        )

    def test_target_is_runnable_and_ordered(self):
        # A target in MAKE_TARGETS but absent from TARGET_ORDER is selected and
        # then dropped by the ordering filter, which fails open silently.
        self.assertIn("ze-learned-staleness", MAKE_TARGETS)
        self.assertIn("ze-learned-staleness", TARGET_ORDER)


if __name__ == "__main__":
    unittest.main()
