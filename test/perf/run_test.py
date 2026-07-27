#!/usr/bin/env python3
"""Unit tests for the perf runner's report-writing guards.

These cover ONE incident, from both of its directions: a `make ze-perf-bench`
run destroyed the committed docs/performance.md.

  1. The generator failed and the destination had already been truncated by
     `open(dest, "w")`, so the document was left at zero bytes. Every byte of a
     published, source-anchored comparison document was gone, and the run still
     exited reporting the benchmarks it had passed.

  2. A DUT-filtered run (PERF_DUT="ze bird") regenerated the same whole-fleet
     document from two results, silently dropping the frr/gobgp/rustbgpd/
     rustybgp/freertr/openbgpd rows.

  3. Underneath both: `ze-perf report --doc` emits ONE undated section, while
     the committed document is hand-curated and carries dated historical
     sections (2026-04-22 through 2026-06-05) the generator cannot reproduce.
     So a run overwriting that file destroys history even when the run is
     complete and every command succeeds.

All three are silent-data-loss shapes: nothing failed loudly, and the damage was
only visible by diffing a file nobody thought the benchmark wrote.
"""

import ast
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import run  # noqa: E402


class TestGenerateToFile(unittest.TestCase):
    """VALIDATES: a failing generator never destroys the existing destination.

    PREVENTS: the truncate-first/ignore-exit-code pattern that emptied
    docs/performance.md to zero bytes.
    """

    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.dest = os.path.join(self.dir.name, "performance.md")
        with open(self.dest, "w") as f:
            f.write("EXISTING CONTENT\n")
        self.addCleanup(self.dir.cleanup)

    def test_failure_leaves_destination_untouched(self):
        ok = run.generate_to_file(
            [sys.executable, "-c", "import sys; sys.stdout.write('x'); sys.exit(1)"],
            self.dest,
        )
        self.assertFalse(ok, "a non-zero generator exit must report failure")
        with open(self.dest) as f:
            self.assertEqual(
                f.read(),
                "EXISTING CONTENT\n",
                "existing document must survive a failed generator",
            )

    def test_failure_leaves_no_temp_file_behind(self):
        run.generate_to_file(
            [sys.executable, "-c", "import sys; sys.exit(2)"], self.dest
        )
        self.assertEqual(
            sorted(os.listdir(self.dir.name)),
            ["performance.md"],
            "the .new scratch file must not be left behind",
        )

    def test_success_replaces_content(self):
        ok = run.generate_to_file(
            [sys.executable, "-c", "print('NEW REPORT')"],
            self.dest,
        )
        self.assertTrue(ok)
        with open(self.dest) as f:
            self.assertEqual(f.read(), "NEW REPORT\n")

    def test_success_creates_missing_destination(self):
        dest = os.path.join(self.dir.name, "nested", "report.html")
        ok = run.generate_to_file([sys.executable, "-c", "print('HTML')"], dest)
        self.assertTrue(ok)
        with open(dest) as f:
            self.assertEqual(f.read(), "HTML\n")


class TestCommittedDocIsNeverWritten(unittest.TestCase):
    """VALIDATES: no report-generation call in the runner targets the committed
    docs/performance.md; the snapshot goes to scratch instead.

    PREVENTS: a benchmark run overwriting a hand-curated document whose dated
    historical sections `ze-perf report --doc` cannot reproduce -- the failure
    that survives every other guard here, because it needs no error at all.

    Asserted over the source's AST rather than by running main(), which would
    need Docker and a full fleet to reach the report stage at all.
    """

    def setUp(self):
        path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "run.py")
        with open(path) as f:
            self.tree = ast.parse(f.read())

    def test_no_generation_call_targets_the_committed_doc(self):
        offenders = []
        for node in ast.walk(self.tree):
            if not isinstance(node, ast.Call):
                continue
            if not isinstance(node.func, ast.Name):
                continue
            if node.func.id not in ("generate_to_file", "open"):
                continue
            for arg in node.args:
                if isinstance(arg, ast.Name) and arg.id == "perf_doc":
                    offenders.append((node.func.id, node.lineno))
        self.assertEqual(
            offenders,
            [],
            "docs/performance.md must never be a write target of a benchmark run; "
            f"found {offenders}",
        )

    def test_snapshot_is_written_under_the_scratch_dir(self):
        # The snapshot destination must derive from RUN_DIR (tmp/perf-run), so
        # the run always has somewhere safe to put its generated section.
        self.assertTrue(
            run.RUN_DIR.startswith(os.path.join(run.PROJECT_ROOT, "tmp")),
            f"RUN_DIR must live under tmp/, got {run.RUN_DIR}",
        )


class TestUnmeasuredDuts(unittest.TestCase):
    """VALIDATES: a partial run is reported as partial, so a snapshot built from
    a subset is labelled rather than passed off as a fleet comparison.

    PREVENTS: `PERF_DUT="ze bird"` results being read as the whole fleet.
    """

    def _results(self, *names):
        return [os.path.join("test", "perf", "results", f"{n}.json") for n in names]

    def test_partial_run_reports_the_absent_duts(self):
        missing = run.unmeasured_duts(self._results("ze", "bird"))
        self.assertIn("frr", missing)
        self.assertIn("gobgp", missing)
        self.assertNotIn("ze", missing)
        self.assertNotIn("bird", missing)

    def test_full_fleet_run_reports_nothing_missing(self):
        every = self._results(*[d["name"] for d in run.DUTS])
        self.assertEqual(run.unmeasured_duts(every), [])

    def test_propagation_result_is_not_mistaken_for_a_dut(self):
        # ze emits an extra ze-propagation.json; it is a second measurement of
        # the ze DUT, not a DUT of its own, and must not make a full run look
        # partial or a partial run look full.
        every = self._results(*[d["name"] for d in run.DUTS]) + self._results(
            "ze-propagation"
        )
        self.assertEqual(run.unmeasured_duts(every), [])

    def test_empty_run_reports_every_dut(self):
        self.assertEqual(run.unmeasured_duts([]), [d["name"] for d in run.DUTS])


if __name__ == "__main__":
    unittest.main()
