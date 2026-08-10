#!/usr/bin/env python3
"""Unit tests for the testing-health page generator.

Discovered and executed by TestPythonUnitTests (scripts/dev/python_tests_test.go),
which globs scripts/dev/*_test.py. No make target is needed; a test file that
nothing invokes reads as coverage while providing none, which is precisely the
defect the tool under test exists to report.
"""

from __future__ import annotations

import json
import os
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
import testing_health as th

LEDGER_HEAD = textwrap.dedent(
    """\
    # RFC Requirement Ledger

    ## Coverage by RFC

    | RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | Nightly-only | State |
    |---|---|---|---|---|---|---|---|---|
    """
)


def write(root: Path, rel: str, body: str) -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(body, encoding="utf-8")


class TestRatio(unittest.TestCase):
    """A ratio must always carry the parts it was computed from.

    VALIDATES: spec AC-5.
    PREVENTS: the improvement-by-shrinking-denominator failure, where a score
    rises because the hard cases stopped being counted.
    """

    def test_ratio_carries_denominator(self):
        r = th.ratio(3, 4)
        self.assertEqual(r["numerator"], 3)
        self.assertEqual(r["denominator"], 4)
        self.assertEqual(r["percent"], 75.0)

    def test_zero_denominator_yields_no_percent(self):
        r = th.ratio(0, 0)
        self.assertIsNone(r["percent"], "a percentage of nothing must not read as 0%")
        self.assertEqual(r["denominator"], 0)


class TestRfcLedgerParse(unittest.TestCase):
    """The ledger is generated, so its format can change under us.

    VALIDATES: spec AC-3, AC-17.
    PREVENTS: a silent zero becoming the published headline after a format change.
    """

    def test_parses_coverage_table(self):
        """The fixture now uses REAL requirement lines.

        The split is counted per gated-level requirement line, not by grepping
        `{kind}` across whole files. The old fixture put both annotations on one
        prefix-less line, which the level-aware parser correctly ignores; that
        loose shape is exactly what let MAY/SHOULD/OPTIONAL annotations leak in
        and made the published split overshoot the remainder by 20.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                th.RFC_LEDGER,
                LEDGER_HEAD
                + "| `rfc1` | 10 | 4 | 0 | 6 | 0 | 0 | 0 | **enrolled** |\n"
                + "| `rfc2` | 5 | 0 | 0 | 5 | 0 | 0 | 0 | **enrolled** |\n",
            )
            write(
                root,
                "rfc/short/rfc1.md",
                "".join(
                    f"- [ ] [RFC1-1-{i}] [MUST] r{i} {{gap: not done}}\n"
                    for i in range(1, 4)
                )
                + "".join(
                    f"- [ ] [RFC1-2-{i}] [MUST] r{i} {{not-applicable: n/a}}\n"
                    for i in range(1, 5)
                )
                # A SHOULD-level annotation must NOT be counted: the ledger's
                # totals cover gated levels only.
                + "- [ ] [RFC1-3-1] [SHOULD] advisory {gap: ignored}\n",
            )
            write(
                root,
                "rfc/short/rfc2.md",
                "".join(
                    f"- [ ] [RFC2-1-{i}] [MUST] r{i} {{single-polarity: positive; z}}\n"
                    for i in range(1, 5)
                ),
            )
            git_init(root)
            headline, unproven = th.collect_rfc(root)
            self.assertEqual(headline.data["proof_density"]["numerator"], 4)
            self.assertEqual(headline.data["proof_density"]["denominator"], 15)
            self.assertEqual(headline.data["annotations"]["gap"], 3)
            self.assertEqual(headline.data["annotations"]["not-applicable"], 4)
            self.assertEqual(headline.data["annotations"]["single-polarity"], 4)
            # The three parts must partition the remainder exactly: 15 - 4 = 11.
            self.assertEqual(sum(headline.data["annotations"].values()), 11)
            # rfc2 has requirements gated but none proven by a test pair.
            self.assertEqual(unproven.data["unproven"]["numerator"], 1)
            self.assertEqual(unproven.data["unproven"]["denominator"], 2)

    def test_unenrolled_rows_are_not_part_of_the_partition(self):
        """An un-enrolled summary's gated MUSTs are legitimately unproven.

        VALIDATES: the partition is asserted over the ENROLLED population, the
        only one `make ze-rfc-check` enforces "proven in both polarities, or
        annotated" for.
        PREVENTS: the collector raising on the extract-then-enrol intermediate
        state that plan/spec-rfcgate-4-ledger.md mandates. Extracting rfc1035,
        rfc4486 and rfc5301 added 34 gated MUST rows that are not yet enrolled,
        and `2754 gated - 974 proven = 1780` no longer matched a split of 1746.

        The fixture kills BOTH halves of the population filter independently:
        counting the un-enrolled row in the totals makes the remainder 11 against
        a split of 6, and counting the un-enrolled SUMMARY's annotations makes
        the split 8 against a remainder of 6. Either way collect_rfc raises.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                th.RFC_LEDGER,
                LEDGER_HEAD
                + "| `rfc1` | 10 | 4 | 0 | 6 | 0 | 0 | 0 | **enrolled** |\n"
                # Gated MUSTs, none proven, none annotated: exactly what an
                # extracted-but-not-yet-enrolled summary looks like.
                + "| `rfc2` | 5 | 0 | 0 | 2 | 3 | 3 | 0 | backlog |\n",
            )
            write(
                root,
                "rfc/short/rfc1.md",
                "".join(
                    f"- [ ] [RFC1-1-{i}] [MUST] r{i} {{gap: not done}}\n"
                    for i in range(1, 4)
                )
                + "".join(
                    f"- [ ] [RFC1-2-{i}] [MUST] r{i} {{not-applicable: n/a}}\n"
                    for i in range(1, 4)
                )
                + "".join(
                    f"- [ ] [RFC1-3-{i}] [MUST] proven both ways\n" for i in range(1, 5)
                ),
            )
            write(
                root,
                "rfc/short/rfc2.md",
                "".join(
                    f"- [ ] [RFC2-1-{i}] [MUST] r{i} {{single-polarity: positive; z}}\n"
                    for i in range(1, 3)
                )
                + "".join(
                    f"- [ ] [RFC2-2-{i}] [MUST] owes a test\n" for i in range(1, 4)
                ),
            )
            git_init(root)
            headline, unproven = th.collect_rfc(root)
            density = headline.data["proof_density"]
            self.assertEqual(density["denominator"], 10, "un-enrolled gated counted")
            self.assertEqual(density["numerator"], 4)
            annotations = headline.data["annotations"]
            self.assertEqual(
                annotations.get("single-polarity", 0),
                0,
                "an un-enrolled summary's annotations were counted",
            )
            self.assertEqual(sum(annotations.values()), 6, annotations)
            self.assertEqual(headline.data["rfcs_total"], 1)
            self.assertEqual(unproven.data["unproven"]["denominator"], 1)
            self.assertEqual(
                unproven.data["unproven"]["numerator"],
                0,
                "a backlog RFC was reported as an enrolled RFC without proof",
            )

    def test_unproven_list_is_the_whole_population_not_a_display_slice(self):
        """`unproven_rfcs` is gated, so it may never be truncated.

        VALIDATES: structural_facts's promise that one RFC LEAVING the list is a
        detectable event -- true only if every unproven RFC is in it.
        PREVENTS: the list being unified with `worst` next door, which is
        deliberately `unproven[:10]` because it renders as a table. That
        truncation is exactly why the gated fact could not be sourced from
        `worst`: an eleventh RFC earning its first pair would move nothing.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            stems = [f"rfc{i:02d}" for i in range(1, 13)]  # 12 > the top-ten slice
            rows = "".join(
                f"| `{stem}` | 1 | 0 | 0 | 1 | 0 | 0 | 0 | **enrolled** |\n"
                for stem in stems
            )
            write(
                root,
                th.RFC_LEDGER,
                LEDGER_HEAD
                + rows
                # One RFC WITH a pair, to prove it is excluded rather than the
                # list simply being every enrolled row.
                + "| `rfcpaired` | 1 | 1 | 0 | 0 | 0 | 0 | 0 | **enrolled** |\n",
            )
            for stem in stems:
                write(
                    root,
                    f"rfc/short/{stem}.md",
                    f"- [ ] [{stem.upper()}-1-1] [MUST] owes a pair {{gap: not done}}\n",
                )
            write(
                root,
                "rfc/short/rfcpaired.md",
                "- [ ] [RFCPAIRED-1-1] [MUST] proven both ways\n",
            )
            git_init(root)
            headline, unproven = th.collect_rfc(root)
            self.assertEqual(unproven.data["unproven_rfcs"], stems)
            self.assertEqual(
                len(unproven.data["unproven_rfcs"]),
                unproven.data["unproven"]["numerator"],
                "the named list and its own count must agree",
            )
            self.assertNotIn("rfcpaired", unproven.data["unproven_rfcs"])
            # The neighbour it must not be confused with: still a display slice.
            self.assertEqual(len(headline.data["worst"]), 10)

    def test_no_enrolled_row_fails_closed(self):
        """Zero enrolled rows must not read as a vacuously satisfied partition.

        VALIDATES: ai/rules/evidence.md -- the zero-value trap. An
        empty enrolled population means the ledger's State marker changed or the
        row parse broke, never that every requirement is accounted for.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                th.RFC_LEDGER,
                LEDGER_HEAD + "| `rfc1` | 5 | 0 | 0 | 0 | 5 | 5 | 0 | backlog |\n",
            )
            write(root, "rfc/short/rfc1.md", "- [ ] [RFC1-1-1] [MUST] owed\n")
            git_init(root)
            with self.assertRaises(th.CollectError) as ctx:
                th.collect_rfc(root)
            self.assertIn("**enrolled**", str(ctx.exception))

    def test_split_that_is_not_a_partition_fails_closed(self):
        """A split that does not sum to the remainder must not be published.

        PREVENTS: the shipped defect where 855+540+371 was presented as the
        breakdown of a remainder of 1746.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                th.RFC_LEDGER,
                LEDGER_HEAD
                + "| `rfc1` | 10 | 4 | 0 | 6 | 0 | 0 | 0 | **enrolled** |\n",
            )
            write(root, "rfc/short/rfc1.md", "- [ ] [RFC1-1-1] [MUST] r {gap: x}\n")
            git_init(root)
            with self.assertRaises(th.CollectError) as ctx:
                th.collect_rfc(root)
            self.assertIn("non-partition", str(ctx.exception))

    def test_header_drift_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root, th.RFC_LEDGER, "# Ledger\n\n| RFC | Something Else |\n|---|---|\n"
            )
            with self.assertRaises(th.CollectError):
                th.collect_rfc(root)

    def test_missing_ledger_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(th.CollectError):
                th.collect_rfc(Path(tmp))

    def test_zero_rows_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.RFC_LEDGER, LEDGER_HEAD)
            with self.assertRaises(th.CollectError):
                th.collect_rfc(root)


class TestMissingArtifactIsUnknown(unittest.TestCase):
    """An unmeasured metric is `unknown`, never `ok` and never zero.

    VALIDATES: spec AC-6.
    PREVENTS: the sensor-rot failure where a broken collector shows green.
    Relevant because mutation_history.py is advisory and records nothing when
    gomu's report is missing (mutation_history.py returns 0 on a read failure).
    """

    def test_absent_mutation_history_is_unknown(self):
        with tempfile.TemporaryDirectory() as tmp:
            m = th.collect_mutation(Path(tmp))
            self.assertEqual(m.status, th.UNKNOWN)
            self.assertIn("no mutation run", m.detail.lower())

    def test_empty_mutation_history_is_unknown(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.MUTATION_HISTORY, "")
            self.assertEqual(th.collect_mutation(root).status, th.UNKNOWN)

    def test_corrupt_mutation_history_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.MUTATION_HISTORY, "{not json}\n")
            with self.assertRaises(th.CollectError):
                th.collect_mutation(root)

    def test_unknown_is_rendered_above_warn(self):
        """Drive the ORDERING through the renderer, not by restating the dict."""
        page = th.render_markdown(
            [
                th.Metric("w", "Q1", "warned metric", th.WARN, "1", action="a"),
                th.Metric(
                    "u", "Q1", "unmeasured metric", th.UNKNOWN, "unknown", action="b"
                ),
            ],
            {"history": []},
        )
        self.assertLess(page.index("unmeasured metric"), page.index("warned metric"))


class TestMutationAggregation(unittest.TestCase):
    """Only the newest sample per package counts, so a re-measured package is
    not double counted into the totals.

    VALIDATES: the kill-rate denominator means what it says.
    PREVENTS: a package measured twice inflating both halves of the ratio.
    """

    def test_latest_sample_per_package_wins(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rows = [
                {"package": "a", "mutants": 10, "killed": 1, "score": 10.0},
                {"package": "a", "mutants": 10, "killed": 9, "score": 90.0},
                {"package": "b", "mutants": 10, "killed": 5, "score": 50.0},
            ]
            write(
                root, th.MUTATION_HISTORY, "\n".join(json.dumps(r) for r in rows) + "\n"
            )
            m = th.collect_mutation(root)
            self.assertEqual(m.data["packages_measured"], 2)
            self.assertEqual(m.data["kill_rate"]["denominator"], 20)
            self.assertEqual(m.data["kill_rate"]["numerator"], 14)
            self.assertEqual(m.data["samples"], 3)


class TestKnownFailures(unittest.TestCase):
    """The live-debt figure is folded from the sharded plan/known-failures/.

    The single tracked file was sharded into a directory
    (spec-fixit-shared-plan-file-contention) so concurrent sessions never
    cross-commit each other's rows: one file per LIVE failure, RESOLVED.md
    archives the history, README.md holds the logging instructions.

    VALIDATES: AC-5' -- live = shard files (excluding README.md/RESOLVED.md),
    resolved = `### ` entries in RESOLVED.md.
    PREVENTS: reporting debt already paid off, the mirror of the inflated
    test-count error this page exists to correct; and the sensor-rot failure
    where an absent input reads as a healthy zero.
    """

    def _write_dir(self, root, shards, resolved_titles):
        for name, body in shards.items():
            write(root, f"plan/known-failures/{name}", body)
        write(root, "plan/known-failures/README.md", "# Known Failures\n\nlog here\n")
        archive = "# Resolved\n\n" + "\n".join(
            f"### {t}\n\nsome body\n" for t in resolved_titles
        )
        write(root, "plan/known-failures/RESOLVED.md", archive)

    def test_live_shards_counted_resolved_archived(self):
        """One shard file per live failure; RESOLVED.md entries are not counted."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._write_dir(
                root,
                {
                    "ze-unit-test-pkg-a.md": "### `pkg/a` -- flaky under load\n\nbody\n",  # <!-- doc-links: ignore (fixture package name, not a tree path) -->
                    "reload-pkg-d.md": "### `pkg/d` -- deterministic panic\n\nbody\n",  # <!-- doc-links: ignore (fixture package name, not a tree path) -->
                },
                ["`pkg/b` -- fixed", "`pkg/c` -- long gone", "`pkg/e` -- resolved"],  # <!-- doc-links: ignore (fixture package name, not a tree path) -->
            )
            m = th.collect_known_failures(root)
            self.assertEqual(m.data["live"], 2)
            self.assertEqual(m.data["resolved"], 3)
            self.assertEqual(m.value, "2")
            self.assertEqual(m.status, th.WARN)

    def test_readme_and_resolved_are_not_live(self):
        """A directory with no live shard is zero live debt, and OK, not UNKNOWN."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._write_dir(root, {}, ["`pkg/x` -- gone"])  # <!-- doc-links: ignore (fixture package name, not a tree path) -->
            m = th.collect_known_failures(root)
            self.assertEqual(m.data["live"], 0)
            self.assertEqual(m.data["resolved"], 1)
            self.assertEqual(m.status, th.OK)

    def test_absent_directory_is_unknown(self):
        """An unmeasured input fails closed to UNKNOWN, never a healthy zero."""
        with tempfile.TemporaryDirectory() as tmp:
            m = th.collect_known_failures(Path(tmp))
            self.assertEqual(m.status, th.UNKNOWN)
            self.assertEqual(m.value, "unknown")

    def test_struck_live_shard_counts_resolved(self):
        """Defensive: a shard whose heading is struck through is not live debt.

        The model never writes one, but a stray strike must not inflate live
        debt. Removing the strike check in collect_known_failures fails this.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._write_dir(
                root,
                {
                    "live-one.md": "### `pkg/a` -- flaky under load\n\nbody\n",  # <!-- doc-links: ignore (fixture package name, not a tree path) -->
                    "stray-struck.md": "### ~~`pkg/z` -- fixed in place~~ -- FIXED\n\nbody\n",  # <!-- doc-links: ignore (fixture package name, not a tree path) -->
                },
                ["`pkg/c` -- long gone"],  # <!-- doc-links: ignore (fixture package name, not a tree path) -->
            )
            m = th.collect_known_failures(root)
            self.assertEqual(m.data["live"], 1, "a struck shard must not count as live")
            self.assertEqual(m.data["resolved"], 2, "struck shard + 1 archived entry")


class TestRatchetStatus(unittest.TestCase):
    """A count at its agreed floor is enforced debt, not a fresh problem.

    VALIDATES: the page does not cry wolf on every run until debt is fully paid.
    PREVENTS: an amber wall, which is as unreadable as a green one.
    """

    def test_at_floor_is_ok(self):
        self.assertEqual(th._ratchet_status(12, 12), th.OK)

    def test_below_floor_is_ok(self):
        self.assertEqual(th._ratchet_status(5, 12), th.OK)

    def test_above_floor_warns(self):
        self.assertEqual(th._ratchet_status(13, 12), th.WARN)

    def test_no_floor_falls_back_to_presence(self):
        self.assertEqual(th._ratchet_status(1, None), th.WARN)
        self.assertEqual(th._ratchet_status(0, None), th.OK)


class TestSparkline(unittest.TestCase):
    """A trend line needs enough samples to mean anything.

    VALIDATES: spec AC-15.
    PREVENTS: three points rendered as a confident direction.
    """

    def test_too_few_points_draws_nothing(self):
        self.assertEqual(th.sparkline([1]), "")
        self.assertEqual(th.sparkline([]), "")

    def test_svg_states_its_sample_count(self):
        svg = th.sparkline([1, 2, 3, 4])
        self.assertTrue(svg.startswith("<svg"))
        self.assertIn("4 samples", svg)
        self.assertIn("polyline", svg)

    def test_flat_series_does_not_divide_by_zero(self):
        svg = th.sparkline([7, 7, 7, 7])
        self.assertIn("polyline", svg)

    def test_trend_section_reports_insufficient_data(self):
        metrics = [
            th.Metric("m", "Q1", "label", th.OK, "1 / 2", action="do a thing"),
        ]
        page = th.render_markdown(metrics, {"history": [{"assert_nothing": 1}]})
        self.assertIn("Insufficient data", page)


class TestRenderContract(unittest.TestCase):
    """Rendering rules that keep the page an instrument, not a scoreboard."""

    def test_output_carries_no_wall_clock(self):
        """VALIDATES: spec AC-1/AC-2. PREVENTS: a staleness gate that flaps."""
        metrics = [th.Metric("m", "Q1", "label", th.OK, "1 / 2", action="do a thing")]
        page = th.render_markdown(metrics, {"history": []})
        import datetime

        year = datetime.datetime.now(datetime.timezone.utc).strftime("%Y")
        # A generated timestamp would put the current year in the page body.
        self.assertNotIn(f"{year}-", page)

    def test_deterministic_output(self):
        """VALIDATES: spec AC-2."""
        metrics = [
            th.Metric(
                "b",
                "Q2",
                "second",
                th.WARN,
                "1",
                action="x",
                data={"worst": [{"k": 2}]},
            ),
            th.Metric("a", "Q1", "first", th.OK, "2", action="y"),
        ]
        first = th.render_markdown(metrics, {"history": []})
        second = th.render_markdown(metrics, {"history": []})
        self.assertEqual(first, second)

    def test_exceptions_are_listed_before_healthy_detail(self):
        """PREVENTS: a green wall where problems sit below the fold."""
        metrics = [
            th.Metric("ok", "Q1", "healthy metric", th.OK, "1", action="none"),
            th.Metric("bad", "Q1", "broken metric", th.WARN, "2", action="fix it"),
        ]
        page = th.render_markdown(metrics, {"history": []})
        attention = page.index("## Needs attention")
        self.assertLess(attention, page.index("## Sensitivity"))
        # The failing metric is named in the attention table, above the detail.
        self.assertLess(page.index("broken metric"), page.index("## Sensitivity"))

    def test_every_metric_states_an_action(self):
        """PREVENTS: decoration. A metric with no action is not actionable."""
        metrics = [th.Metric("m", "Q1", "label", th.WARN, "1", action="do the thing")]
        page = th.render_markdown(metrics, {"history": []})
        self.assertIn("do the thing", page)


class TestInventoryBoundary(unittest.TestCase):
    """Counts must exclude third-party module trees.

    VALIDATES: spec AC-4.
    PREVENTS: the published-count error this whole page was built to correct,
    where vendored dependency tests inflated the total roughly sixfold.
    """

    def test_module_cache_is_not_counted(self):
        """Each decoy sits where a filter must actually reject it.

        An earlier version put the only decoy under gokrazy/, which is outside
        TEST_ROOTS entirely -- so the test passed even with the vendor/testdata
        filter deleted. These decoys live INSIDE a test root, so removing the
        filter fails this test.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                "internal/example/a_test.go",
                "package example\nfunc TestReal(t *testing.T) {}\n",
            )
            write(
                root,
                "gokrazy/modcache/dep/b_test.go",
                "package dep\nfunc TestModCache(t *testing.T) {}\n",
            )
            write(
                root,
                "internal/example/testdata/c_test.go",
                "package testdata\nfunc TestFixture(t *testing.T) {}\n",
            )
            write(
                root,
                "internal/example/vendor/d_test.go",
                "package vendored\nfunc TestVendored(t *testing.T) {}\n",
            )
            git_init(root)
            m = th.collect_inventory(root)
            self.assertEqual(m.data["counts"]["test_funcs"], 1, m.data["counts"])
            self.assertIn("third-party", m.detail)

    def test_empty_tree_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(th.CollectError):
                th.collect_inventory(Path(tmp))


def rfc_unproven_metric(**over):
    """A well-formed `rfc-unproven` snapshot metric: list and count agreeing.

    Every structural-fact test needs the OTHER facts well-formed, or a guard
    fires for the wrong reason and the test stops discriminating the guard it
    names.
    """
    m = {
        "key": "rfc-unproven",
        "status": th.WARN,
        "value": "2 / 3",
        "unproven": th.ratio(2, 3),
        "unproven_rfcs": ["rfc1", "rfc2"],
    }
    m.update(over)
    return m


def tag_orphan_metric(**over):
    """A well-formed `tag-orphan` snapshot metric: `orphans` agreeing with
    `orphan_count`, which is what collect_inert writes."""
    m = {
        "key": "tag-orphan",
        "status": th.OK,
        "value": "1 (floor 1)",
        "orphan_count": 1,
        "orphans": [{"file": "internal/a/a_test.go", "requires": "ze_x"}],
    }
    m.update(over)
    return m


def fake_metrics(assert_nothing: int, tag_orphan: int):
    """Minimal metric set with the two keys kpi_row needs for the baseline."""
    return [
        th.Metric(
            "rfc-proof-density",
            "Q2",
            "rfc",
            th.OK,
            "1 / 2",
            action="x",
            data={"proof_density": th.ratio(1, 2)},
        ),
        th.Metric(
            "assert-nothing",
            "Q1",
            "inert",
            th.OK,
            str(assert_nothing),
            action="x",
            data={"inert": th.ratio(assert_nothing, 100)},
        ),
        th.Metric(
            "tag-orphan",
            "Q3",
            "orphans",
            th.OK,
            str(tag_orphan),
            action="x",
            data={"orphan_count": tag_orphan},
        ),
        th.Metric("mutation", "Q1", "mutation", th.UNKNOWN, "unknown", action="x"),
        th.Metric(
            "ci-sleeps", "Q1", "sleeps", th.OK, "1", action="x", data={"actual": 1}
        ),
    ]


class TestBaselineOnlyTightens(unittest.TestCase):
    """The floors may fall and never rise.

    VALIDATES: a regression cannot be laundered into the baseline by running the
    generator, which is the single most abusable behaviour in this file.
    PREVENTS: `make ze-test-health` quietly blessing a worse tree.
    """

    def _baseline(self, root):
        return json.loads((root / th.BASELINE).read_text())

    def test_higher_count_does_not_raise_the_floor(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.BASELINE, json.dumps({"assert-nothing": 5, "tag-orphan": 2}))
            th.tighten_baseline(root, fake_metrics(99, 42))
            self.assertEqual(
                self._baseline(root), {"assert-nothing": 5, "tag-orphan": 2}
            )

    def test_lower_count_tightens_the_floor(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.BASELINE, json.dumps({"assert-nothing": 5, "tag-orphan": 2}))
            changed = th.tighten_baseline(root, fake_metrics(3, 0))
            self.assertTrue(changed)
            self.assertEqual(
                self._baseline(root), {"assert-nothing": 3, "tag-orphan": 0}
            )

    def test_unchanged_counts_report_no_change(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.BASELINE, json.dumps({"assert-nothing": 5, "tag-orphan": 2}))
            self.assertFalse(th.tighten_baseline(root, fake_metrics(5, 2)))

    def test_missing_key_is_an_error_not_a_new_floor(self):
        """A defaulted floor becomes whatever today's count is: a silent raise."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.BASELINE, json.dumps({"tag-orphan": 2}))
            with self.assertRaises(th.CollectError):
                th.tighten_baseline(root, fake_metrics(99, 2))

    def test_non_integer_floor_is_an_error(self):
        for bad in (
            '{"assert-nothing": null, "tag-orphan": 0}',
            '{"assert-nothing": "5", "tag-orphan": 0}',
        ):
            with tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                write(root, th.BASELINE, bad)
                with self.assertRaises(th.CollectError):
                    th.tighten_baseline(root, fake_metrics(1, 0))

    def test_corrupt_baseline_is_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.BASELINE, "{not json}")
            with self.assertRaises(th.CollectError):
                th.tighten_baseline(root, fake_metrics(1, 0))

    def test_baseline_list_is_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.BASELINE, "[1, 2, 3]")
            with self.assertRaises(th.CollectError):
                th.tighten_baseline(root, fake_metrics(1, 0))


class TestAdoptionBuckets(unittest.TestCase):
    """VALIDATES: spec AC-18. PREVENTS: the published package count exceeding the
    number of packages that exist."""

    def test_interleaved_subdirectory_does_not_double_count(self):
        """sorted() puts a subdirectory between two files of its parent, so a
        single-slot sentinel re-entered the parent and counted it twice."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for rel in (
                "internal/a/b_test.go",
                "internal/a/sub/c_test.go",
                "internal/a/z_test.go",
            ):
                write(root, rel, "package a\n")
            git_init(root)
            m = th.collect_adoption(root)
            total = sum(b["packages"] for b in m.data["buckets"].values())
            self.assertEqual(
                total,
                2,
                f"expected internal/a and internal/a/sub, got {m.data['buckets']}",
            )

    def test_fuzz_and_rfc_columns_count_each_package_once(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                "internal/a/b_test.go",
                "package a\n\nfunc FuzzOne(f *testing.F) {}\n",
            )
            write(root, "internal/a/sub/c_test.go", "package sub\n")
            write(
                root,
                "internal/a/z_test.go",
                "package a\n\nfunc FuzzTwo(f *testing.F) {}\n",
            )
            git_init(root)
            m = th.collect_adoption(root)
            total_fuzz = sum(b["with_fuzz"] for b in m.data["buckets"].values())
            self.assertEqual(total_fuzz, 1, m.data["buckets"])

    def test_ci_presence_is_reported(self):
        """AC-18 names all THREE techniques; the .ci column was never written."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                "internal/a/a_test.go",
                "package a\n\nfunc TestA(t *testing.T) {}\n",
            )
            write(root, "test/plugin/one.ci", "# a functional scenario\n")
            write(root, "test/plugin/two.ci", "# another in the same directory\n")
            git_init(root)
            m = th.collect_adoption(root)
            total_ci = sum(b.get("with_ci", 0) for b in m.data["buckets"].values())
            # Two .ci files in one directory count that directory ONCE.
            self.assertEqual(total_ci, 1, m.data["buckets"])
            self.assertIn("with_ci", next(iter(m.data["buckets"].values())))


class TestTrackedFilesDriveThePage(unittest.TestCase):
    """VALIDATES: spec AC-2 -- the page reflects committed state.

    PREVENTS: an untracked work-in-progress test moving the published numbers, so
    a clean CI checkout disagrees with the committed page and the staleness gate
    reds for everyone.
    """

    def test_untracked_test_file_is_not_counted(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                "internal/a/tracked_test.go",
                "package a\n\nfunc TestA(t *testing.T) {}\n",
            )
            git_init(root)
            before = th.collect_inventory(root).data["counts"]["test_funcs"]

            write(
                root,
                "internal/a/scratch_test.go",
                "package a\n\nfunc TestB(t *testing.T) {}\n",
            )
            th._TRACKED_CACHE.clear()
            after = th.collect_inventory(root).data["counts"]["test_funcs"]

            self.assertEqual(
                before, after, "an untracked file changed the published count"
            )

    def test_no_git_repository_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            th._TRACKED_CACHE.clear()
            with self.assertRaises(th.CollectError):
                th.tracked_files(Path(tmp))


class TestNegativeAssertTokens(unittest.TestCase):
    """The metric must count error EXPECTATIONS, not happy-path setup guards.

    VALIDATES: the label matches what is measured.
    PREVENTS: publishing a number that means close to the opposite of its name.
    """

    def test_error_expectation_tokens_match(self):
        for src in (
            "wantErr: true",
            "require.Error(t, err)",
            "errors.Is check via ErrorIs(t, err, x)",
        ):
            self.assertTrue(th.NEGATIVE_ASSERT.search(src), src)

    def test_setup_guard_does_not_match(self):
        src = 'if err != nil { t.Fatalf("setup: %v", err) }'
        self.assertIsNone(
            th.NEGATIVE_ASSERT.search(src),
            "a setup guard asserts the happy path and must not count",
        )

    def test_comment_mentioning_a_token_does_not_count(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                "internal/a/a_test.go",
                "package a\n\n// wantErr is unused here\nfunc TestA(t *testing.T) {}\n",
            )
            for i in range(5):
                write(root, f"internal/a/pad{i}_test.go", "package a\n")
            git_init(root)
            m = th.collect_negative_tests(root)
            self.assertEqual(
                m.data["overall"]["numerator"], 0, "a comment was counted as coverage"
            )


class TestSleepBaselineRobustness(unittest.TestCase):
    """VALIDATES: ai/rules/evidence.md."""

    def test_non_integer_baseline_is_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.SLEEP_BASELINE, "125 # allowed\n")
            write(root, "internal/a/a_test.go", "package a\n")
            git_init(root)
            with self.assertRaises(th.CollectError):
                th.collect_sleep_ratchet(root)


class TestHistoryRobustness(unittest.TestCase):
    """VALIDATES: spec AC-14 and fail-closed parsing of the KPI series."""

    def test_corrupt_history_line_is_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.HISTORY, '{"ok": 1}\n{not json}\n')
            with self.assertRaises(th.CollectError):
                th.load_history(root)

    def test_absent_history_is_empty_not_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(th.load_history(Path(tmp)), [])


class TestMutationRobustness(unittest.TestCase):
    """VALIDATES: spec AC-6 -- unmeasured is `unknown`, never a green zero."""

    def test_null_score_does_not_crash(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rows = [
                {"package": "a", "mutants": 4, "killed": 2, "score": None},
                {"package": "b", "mutants": 4, "killed": 4, "score": 100.0},
            ]
            write(
                root, th.MUTATION_HISTORY, "\n".join(json.dumps(r) for r in rows) + "\n"
            )
            m = th.collect_mutation(root)
            self.assertEqual(m.data["kill_rate"]["denominator"], 8)

    def test_row_without_a_package_is_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                th.MUTATION_HISTORY,
                json.dumps({"mutants": 1, "killed": 1}) + "\n",
            )
            with self.assertRaises(th.CollectError):
                th.collect_mutation(root)

    def test_zero_mutants_is_unknown_not_zero_percent(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(
                root,
                th.MUTATION_HISTORY,
                json.dumps({"package": "a", "mutants": 0, "killed": 0}) + "\n",
            )
            self.assertEqual(th.collect_mutation(root).status, th.UNKNOWN)


class TestWriteCheckRoundTrip(unittest.TestCase):
    """`--write` then `--check` must be green, including when a floor tightens.

    VALIDATES: spec AC-1, AC-2, AC-12.
    PREVENTS: the defect where the page was rendered BEFORE the baseline moved,
    so the ratchet's own remediation ("run make ze-test-health to tighten it")
    left the page stale and failed the very next verify. Nothing drove the write
    -> check round trip, which is why it shipped.
    """

    def _repo(self, tmp: str) -> Path:
        """A fixture repository complete enough for every collector."""
        root = Path(tmp)
        write(
            root,
            th.RFC_LEDGER,
            LEDGER_HEAD + "| `rfc1` | 10 | 4 | 0 | 6 | 0 | 0 | 0 | **enrolled** |\n",
        )
        write(root, "rfc/short/rfc1.md", "{gap: x} {not-applicable: y}\n")
        write(
            root, "internal/a/a_test.go", "package a\n\nfunc TestA(t *testing.T) {}\n"
        )
        write(root, th.SLEEP_BASELINE, "10\n")
        write(root, "plan/known-failures/README.md", "# Known Failures\n\nlog here\n")
        git_init(root)
        return root

    def test_write_then_check_is_green_when_a_floor_tightens(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            # Deliberately slack floors, the exact state the ratchet reports as
            # "baseline is slack -- run make ze-test-health to tighten it".
            write(
                root, th.BASELINE, json.dumps({"assert-nothing": 500, "tag-orphan": 50})
            )

            metrics = [
                th.Metric(
                    "rfc-proof-density",
                    "Q2",
                    "rfc",
                    th.OK,
                    "1 / 2",
                    action="x",
                    data={"proof_density": th.ratio(1, 2)},
                ),
                th.Metric(
                    "assert-nothing",
                    "Q1",
                    "inert",
                    th.OK,
                    "3",
                    action="x",
                    data={"inert": th.ratio(3, 100)},
                ),
                th.Metric(
                    "tag-orphan",
                    "Q3",
                    "orphans",
                    th.OK,
                    "1",
                    action="x",
                    data={"orphan_count": 1},
                ),
                th.Metric(
                    "mutation", "Q1", "mutation", th.UNKNOWN, "unknown", action="x"
                ),
                th.Metric(
                    "ci-sleeps",
                    "Q1",
                    "sleeps",
                    th.OK,
                    "1",
                    action="x",
                    data={"actual": 1},
                ),
            ]
            changed = th.tighten_baseline(root, metrics)
            self.assertTrue(changed, "a slack floor should tighten")

            floors = json.loads((root / th.BASELINE).read_text())
            self.assertEqual(floors, {"assert-nothing": 3, "tag-orphan": 1})

            # Second pass must be a fixed point, or --write is not idempotent.
            self.assertFalse(
                th.tighten_baseline(root, metrics),
                "tightening twice moved the floor again; --write is not idempotent",
            )

    def test_missing_baseline_file_is_refused_without_bootstrap(self):
        """Deleting the baseline must not mint today's counts as the floors."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with self.assertRaises(th.CollectError) as ctx:
                th.tighten_baseline(root, fake_metrics(999, 77))
            self.assertIn("--bootstrap-baseline", str(ctx.exception))
            self.assertFalse((root / th.BASELINE).exists())

    def test_bootstrap_creates_the_baseline_deliberately(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            th.tighten_baseline(root, fake_metrics(7, 2), bootstrap=True)
            self.assertEqual(
                json.loads((root / th.BASELINE).read_text()),
                {"assert-nothing": 7, "tag-orphan": 2},
            )


class TestQualityFloor(unittest.TestCase):
    """A higher-is-better metric warns only on REGRESSION below its locked best.

    PREVENTS: the green-wall-from-the-other-side failure, where three bare
    thresholds (50/75/40) put five rows in the attention table permanently, so
    the table stops carrying information. The status is now a regression signal
    against a committed floor, exactly like the sensitivity ratchet.
    """

    def _metric(self, key, percent):
        parts = {
            "rfc-proof-density": "proof_density",
            "mutation": "kill_rate",
            "negative-tests": "overall",
        }
        return th.Metric(
            key, "Q2", key, th.OK, "x", data={parts[key]: {"percent": percent}}
        )

    def test_no_floor_yet_is_ok(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(th.quality_status(Path(tmp), "mutation", 12.0), th.OK)

    def test_at_or_above_floor_is_ok(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.QUALITY_BASELINE, json.dumps({"mutation": 60.0}))
            self.assertEqual(th.quality_status(root, "mutation", 60.0), th.OK)
            self.assertEqual(th.quality_status(root, "mutation", 72.0), th.OK)

    def test_below_floor_warns(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.QUALITY_BASELINE, json.dumps({"mutation": 60.0}))
            self.assertEqual(th.quality_status(root, "mutation", 45.0), th.WARN)

    def test_float_wobble_does_not_warn(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.QUALITY_BASELINE, json.dumps({"mutation": 60.4}))
            self.assertEqual(th.quality_status(root, "mutation", 60.35), th.OK)

    def test_none_percent_is_unknown(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(th.quality_status(Path(tmp), "mutation", None), th.UNKNOWN)

    def test_tighten_raises_never_lowers(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.QUALITY_BASELINE, json.dumps({"mutation": 60.0}))
            # An improvement locks in.
            self.assertTrue(th.tighten_quality(root, [self._metric("mutation", 70.0)]))
            self.assertEqual(
                json.loads((root / th.QUALITY_BASELINE).read_text())["mutation"], 70.0
            )
            # A regression does NOT lower the floor, so the next status is WARN.
            self.assertFalse(th.tighten_quality(root, [self._metric("mutation", 50.0)]))
            self.assertEqual(
                json.loads((root / th.QUALITY_BASELINE).read_text())["mutation"], 70.0
            )
            self.assertEqual(th.quality_status(root, "mutation", 50.0), th.WARN)

    def test_corrupt_quality_baseline_is_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, th.QUALITY_BASELINE, "{not json}")
            with self.assertRaises(th.CollectError):
                th.read_quality_floors(root)


class TestNegativeTestAreaBuckets(unittest.TestCase):
    """Areas must be directories, never individual files.

    PREVENTS: 117 of 318 "areas" being single file paths, each with one file, so
    the >=5 filter dropped them all and whole trees could never appear in the
    table the metric's own action tells you to act on.
    """

    def test_area_is_a_directory(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for i in range(6):
                write(root, f"internal/analyze/f{i}_test.go", "package analyze\n")
            git_init(root)
            m = th.collect_negative_tests(root)
            areas = [row["area"] for row in m.data["worst"]]
            self.assertIn("internal/analyze", areas, m.data["worst"])
            for area in areas:
                self.assertFalse(
                    area.endswith(".go"), f"area {area!r} is a file, not a subsystem"
                )


class TestDescribeNamesWhatMoved(unittest.TestCase):
    """A gate that prints two long lists has told the reader nothing.

    VALIDATES: _describe's own promise, "name what moved, so the failure is
    diagnosable without a diff tool".
    PREVENTS: the `rfc-unproven` fact -- 36 RFC stems today -- being reported as
    two near-identical 36-item lists, leaving the one that moved to be found by
    eye. It was invisible while the fact was empty; populating it made the
    difference the only part worth printing.
    """

    def test_list_difference_is_named_not_dumped(self):
        committed = [f"rfc{i}" for i in range(1, 15)]
        generated = [r for r in committed if r != "rfc7"] + ["rfc99"]
        lines = th._describe(committed, sorted(generated), "rfc-unproven")
        joined = "\n".join(lines)
        self.assertIn("rfc7", joined)
        self.assertIn("rfc99", joined)
        self.assertNotIn("rfc3", joined, f"unmoved entries were dumped:\n{joined}")

    def test_identical_lists_describe_nothing(self):
        self.assertEqual(th._describe(["a"], ["a"], "x"), [])

    def test_non_list_facts_still_print_both_sides(self):
        lines = th._describe({"a": "ok"}, {"a": "warn"}, "statuses")
        joined = "\n".join(lines)
        self.assertIn("ok", joined)
        self.assertIn("warn", joined)


class TestStructuralFactRfcUnproven(unittest.TestCase):
    """The gated fact must carry the RFCs it names, and refuse to go quiet.

    VALIDATES: structural_facts()'s own promise -- "which enrolled RFCs have no
    test pair -- one leaving the list is a requirement newly proven".
    PREVENTS: the defect this class was written for. The fact read `worst` off
    the `rfc-unproven` metric, a key that metric never carried (a truncated
    top-ten list lived on `rfc-proof-density` instead), so it evaluated to `[]`
    on BOTH sides of every comparison and could not detect anything. A gated
    check that cannot fail reads as coverage while providing none
    (ai/rules/testing.md, Test Sensitivity Ratchets).
    """

    def _metric(self, **over):
        return rfc_unproven_metric(**over)

    def _record(self, *metrics):
        """The metric(s) under test, PLUS a well-formed `tag-orphan` neighbour.

        Every fact in structural_facts is now cross-checked, so a record that
        omits the neighbour raises from the neighbour's guard and the test stops
        proving anything about `rfc-unproven`. The message assertions below pin
        which guard spoke.
        """
        return {"metrics": [*metrics, tag_orphan_metric()]}

    def test_fact_names_every_unproven_rfc(self):
        """The whole set, not a display slice: an RFC ranked 20th being proven is
        the same event as the first one being proven, and must be as detectable."""
        facts = th.structural_facts(self._record(self._metric()))
        self.assertEqual(facts["rfc-unproven"], ["rfc1", "rfc2"])

    def test_empty_is_legal_only_when_the_count_agrees(self):
        """Zero unproven RFCs is the goal state, so `[]` must stay expressible."""
        facts = th.structural_facts(
            self._record(
                self._metric(unproven=th.ratio(0, 3), unproven_rfcs=[], value="0 / 3")
            )
        )
        self.assertEqual(facts["rfc-unproven"], [])

    def test_missing_field_fails_closed(self):
        """A snapshot predating the field, or a renamed one, must speak. Yielding
        `[]` here IS the defect (ai/rules/evidence.md)."""
        m = self._metric()
        del m["unproven_rfcs"]
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(m))
        self.assertIn("rfc-unproven", str(ctx.exception))
        self.assertIn("written before the field existed", str(ctx.exception))

    def test_metric_absent_fails_closed(self):
        """Sourcing the fact from `rfc-proof-density` instead is not a fallback."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(
                self._record(
                    {
                        "key": "rfc-proof-density",
                        "status": th.OK,
                        "worst": [{"rfc": "rfc1", "gated": 9}],
                    }
                )
            )
        self.assertIn("rfc-unproven", str(ctx.exception))

    def test_list_contradicting_its_own_count_fails_closed(self):
        """The cross-check that makes a silent `[]` impossible: the metric counts
        two unproven RFCs, so a list that names none of them is not measurement."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(self._metric(unproven_rfcs=[])))
        self.assertIn("rfc-unproven", str(ctx.exception))
        self.assertIn("have diverged", str(ctx.exception))

    def test_non_list_field_fails_closed(self):
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(self._metric(unproven_rfcs="rfc1")))
        self.assertIn("rfc-unproven", str(ctx.exception))
        self.assertIn("expected a list", str(ctx.exception))

    def test_absent_count_fails_closed(self):
        m = self._metric()
        del m["unproven"]
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(m))
        self.assertIn("rfc-unproven", str(ctx.exception))
        self.assertIn("cannot be checked against its own count", str(ctx.exception))


class TestStructuralFactTagOrphans(unittest.TestCase):
    """The tag-orphan fact must be unable to go quiet, same as `rfc-unproven`.

    VALIDATES: structural_facts()'s own promise -- "which test files no `go test`
    target can build -- a new one means a build tag or a make target just
    stranded a file".
    PREVENTS: the vacuity that shipped one fact over. This fact reads
    `by_key.get("tag-orphan", {}).get("orphans") or []`, which is the DEFECT'S
    EXACT SHAPE: a missing metric, a renamed field, or a non-list all collapse to
    `[]` on BOTH sides of the comparison, and an empty list is indistinguishable
    from "no test file is stranded" -- the goal state. The list is correct today
    (8 orphans), so this is the back-fill ai/rules/testing.md requires, not a bug
    fix: coverage that only grows forward from the date a technique was invented
    is the trap the rule names.

    The cross-check is `orphan_count`, which collect_inert writes from the same
    `len(orphans)`. A list disagreeing with its own counter cannot be
    measurement: it is a stale snapshot, a truncation, or a reader on the wrong
    field.
    """

    def _record(self, *metrics):
        """The metric(s) under test, plus a well-formed `rfc-unproven` neighbour."""
        return {"metrics": [*metrics, rfc_unproven_metric()]}

    def test_fact_names_every_orphan(self):
        facts = th.structural_facts(self._record(tag_orphan_metric()))
        self.assertEqual(facts["tag-orphans"], [("internal/a/a_test.go", "ze_x")])

    def test_empty_is_legal_only_when_the_count_agrees(self):
        """Zero stranded files is the goal state, so `[]` must stay expressible."""
        facts = th.structural_facts(
            self._record(tag_orphan_metric(orphan_count=0, orphans=[], value="0"))
        )
        self.assertEqual(facts["tag-orphans"], [])

    def test_metric_absent_fails_closed(self):
        """`by_key.get("tag-orphan", {})` defaulting to `{}` is the hole: it makes
        a misspelled metric key read as `nothing is stranded`."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record())
        self.assertIn("tag-orphan", str(ctx.exception))
        self.assertIn("has no source", str(ctx.exception))

    def test_missing_field_fails_closed(self):
        """A snapshot predating the field, or a renamed one, has no answer here --
        which is not the same as zero orphans."""
        m = tag_orphan_metric()
        del m["orphans"]
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(m))
        self.assertIn("tag-orphan", str(ctx.exception))
        # Pin THIS branch: the length cross-check downstream would also refuse
        # the record, so a bare assertRaises cannot prove this guard exists.
        self.assertIn("written before the field existed", str(ctx.exception))

    def test_non_list_field_fails_closed(self):
        """A string iterates into characters, every one of which the `isinstance`
        filter drops -- so a wrong type reached `[]` silently."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(tag_orphan_metric(orphans="a_test.go")))
        self.assertIn("tag-orphan", str(ctx.exception))
        self.assertIn("expected a list", str(ctx.exception))

    def test_absent_count_fails_closed(self):
        """With no counter there is nothing to cross-check against, so the list
        cannot be trusted; that is an error, not a reason to skip the check."""
        m = tag_orphan_metric()
        del m["orphan_count"]
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(m))
        self.assertIn("tag-orphan", str(ctx.exception))
        self.assertIn("cannot be checked against its own count", str(ctx.exception))

    def test_non_integer_count_fails_closed(self):
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(tag_orphan_metric(orphan_count="1")))
        self.assertIn("tag-orphan", str(ctx.exception))
        self.assertIn("cannot be checked against its own count", str(ctx.exception))

    def test_list_contradicting_its_own_count_fails_closed(self):
        """The check that makes a silent `[]` impossible: the metric counts three
        stranded files, so a list naming one of them is not measurement."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record(tag_orphan_metric(orphan_count=3)))
        self.assertIn("tag-orphan", str(ctx.exception))
        self.assertIn("have diverged", str(ctx.exception))

    def test_empty_list_against_a_nonzero_count_fails_closed(self):
        """The precise vacuity: `[]` published while the counter says 8."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(
                self._record(tag_orphan_metric(orphans=[], orphan_count=8))
            )
        self.assertIn("tag-orphan", str(ctx.exception))
        self.assertIn("have diverged", str(ctx.exception))


class TestStructuralFactStatuses(unittest.TestCase):
    """Every metric's status is gated, so the reader of it must be unable to
    degenerate into a constant.

    VALIDATES: structural_facts()'s own promise -- "a flip to `warn`, and above
    all to `unknown`, means a collector stopped measuring. Sensor rot is the
    failure mode the page exists to make visible, so it must not be able to land
    silently."
    PREVENTS: the `statuses` analogue of the `rfc-unproven` defect, which is
    strictly worse because it is TOTAL. This fact has no counter of its own, so
    it is cross-checked against two sources independent of its content: the
    status vocabulary (`OK`/`WARN`/`UNKNOWN`, the module's own constants) and the
    cardinality of the metrics list it is derived from. A reader looking at the
    wrong field yields `None` for every metric, which compares EQUAL on both
    sides for any status change whatsoever; a reader on the wrong KEY field
    collapses ten metrics into one dict entry, hiding nine statuses. Neither is
    caught by comparing the two snapshots, because both sides degenerate
    identically -- which is exactly how the shipped defect survived.

    Deliberately NOT a check of the list's length against itself: that is a
    tautology and would read as protection while providing none.
    """

    def _record(self, *metrics):
        return {"metrics": [*metrics, tag_orphan_metric(), rfc_unproven_metric()]}

    def test_fact_maps_every_metric_key_to_its_status(self):
        facts = th.structural_facts(
            self._record({"key": "known-failures", "status": th.UNKNOWN})
        )
        self.assertEqual(
            facts["statuses"],
            {
                "known-failures": th.UNKNOWN,
                "tag-orphan": th.OK,
                "rfc-unproven": th.WARN,
            },
        )

    def test_status_outside_the_vocabulary_fails_closed(self):
        """A value no collector can produce means the reader is on another field."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record({"key": "m", "status": "green"}))
        self.assertIn("statuses", str(ctx.exception))

    def test_absent_status_fails_closed(self):
        """`None` for every metric is what a renamed `status` field produces, and
        it makes the fact compare equal on both sides for ANY status change."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record({"key": "m"}))
        self.assertIn("statuses", str(ctx.exception))

    def test_collapsed_keys_fail_closed(self):
        """Two metrics under one key means eight of ten statuses are unwatched.

        This is what reading the wrong KEY field does: every entry lands under
        `None` and the dict comprehension silently keeps only the last one.
        """
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(
                self._record(
                    {"key": "dup", "status": th.OK}, {"key": "dup", "status": th.WARN}
                )
            )
        self.assertIn("statuses", str(ctx.exception))

    def test_metric_without_a_key_fails_closed(self):
        """A single missing key is invisible to the cardinality check, so the key
        itself is checked too."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts(self._record({"status": th.OK}))
        self.assertIn("statuses", str(ctx.exception))

    def test_empty_metric_list_fails_closed(self):
        """`{}` statuses, `[]` orphans and `[]` unproven RFCs all at once reads as
        a perfectly healthy repository. The precondition is checked before any
        fact, so this does not depend on a sibling guard noticing."""
        with self.assertRaises(th.CollectError) as ctx:
            th.structural_facts({"metrics": []})
        self.assertIn("no metrics", str(ctx.exception))


class TestEntryPoints(unittest.TestCase):
    """Drive do_write / do_check / do_record, not just their helpers.

    VALIDATES: spec AC-1, AC-2, AC-12, AC-14.
    PREVENTS: the gap that shipped twice. `TestWriteCheckRoundTrip` claimed to
    cover the write -> check round trip and only ever called `tighten_baseline`,
    so re-swapping the render/tighten order in `do_write` would not have failed
    a single test. ai/rules/evidence.md: drive the guard from its
    entry point, never the helper alone.
    """

    def _repo(self, tmp: str) -> Path:
        root = Path(tmp)
        # rfc1 has a proven pair; rfc2 has none. The unproven row is what makes
        # the `rfc-unproven` structural fact non-empty, so the round-trip tests
        # below exercise the gate with real content instead of comparing two
        # empty lists -- which is how the vacuity survived this class once.
        write(
            root,
            th.RFC_LEDGER,
            LEDGER_HEAD
            + "| `rfc1` | 4 | 1 | 0 | 3 | 0 | 0 | 0 | **enrolled** |\n"
            + "| `rfc2` | 2 | 0 | 0 | 2 | 0 | 0 | 0 | **enrolled** |\n",
        )
        write(
            root,
            "rfc/short/rfc1.md",
            "- [ ] [RFC1-1-1] [MUST] a {gap: x}\n"
            "- [ ] [RFC1-1-2] [MUST] b {not-applicable: y}\n"
            "- [ ] [RFC1-1-3] [MUST] c {single-polarity: positive; z}\n",
        )
        write(
            root,
            "rfc/short/rfc2.md",
            "- [ ] [RFC2-1-1] [MUST] d {gap: x}\n"
            "- [ ] [RFC2-1-2] [MUST] e {not-applicable: y}\n",
        )
        write(
            root, "internal/a/a_test.go", "package a\n\nfunc TestA(t *testing.T) {}\n"
        )
        write(root, th.SLEEP_BASELINE, "10\n")
        write(root, "plan/known-failures/README.md", "# Known Failures\n\nlog here\n")
        write(root, th.BASELINE, json.dumps({"assert-nothing": 9, "tag-orphan": 9}))
        git_init(root)
        return root

    def _stub_inert(self, monkey):
        """Stand in for the Go gate so these tests do not shell out to `go run`."""
        monkey["orig"] = th.collect_inert

        def fake(root):
            floors = th.read_baseline(root)
            inert = th.Metric(
                "assert-nothing",
                "Q1",
                "Tests with no reachable failure call",
                th._ratchet_status(1, floors.get("assert-nothing")),
                "1 / 10" + th._floor_suffix(floors.get("assert-nothing")),
                action="a",
                data={"inert": th.ratio(1, 10), "worst": []},
            )
            orphan = th.Metric(
                "tag-orphan",
                "Q3",
                "Test files no `go test` target can build",
                th._ratchet_status(0, floors.get("tag-orphan")),
                "0" + th._floor_suffix(floors.get("tag-orphan")),
                action="b",
                data={"orphan_count": 0, "orphans": []},
            )
            return inert, orphan, {}

        th.collect_inert = fake

    def setUp(self):
        self._monkey = {}
        self._stub_inert(self._monkey)

    def tearDown(self):
        th.collect_inert = self._monkey["orig"]
        th._TRACKED_CACHE.clear()

    def test_write_creates_all_three_artifacts_then_check_is_green(self):
        """AC-1 + AC-12: the real entry points, in the real order."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            self.assertEqual(th.do_write(root), 0)
            for rel in (th.PAGE, th.LATEST, th.BASELINE):
                self.assertTrue((root / rel).exists(), f"{rel} was not written")
            self.assertEqual(th.do_check(root), 0)

    def test_write_is_idempotent_even_when_a_floor_tightens(self):
        """AC-2: the defect was rendering the page BEFORE the floor moved."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)  # floors 9/9, measured 1/0 -> both tighten
            th.do_write(root)
            first = (root / th.PAGE).read_text()
            self.assertEqual(
                json.loads((root / th.BASELINE).read_text()),
                {"assert-nothing": 1, "tag-orphan": 0},
            )
            self.assertEqual(th.do_check(root), 0, "page was stale right after --write")
            th.do_write(root)
            self.assertEqual(first, (root / th.PAGE).read_text())

    def test_check_ignores_pure_volume_drift(self):
        """AC-12, revised: counters are published, not gated.

        A byte-exact gate charged a regeneration to ~60% of commits, because
        every added test moves a denominator. That is the "advisory gate
        permanently red" failure this page is built to expose, so the gate now
        discriminates: an event fails it, churn does not.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            record = json.loads((root / th.LATEST).read_text())
            for m in record["metrics"]:
                if m["key"] == "inventory":
                    m["counts"]["test_funcs"] = 999999
                    m["value"] = "999999 test functions"
            (root / th.LATEST).write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
            self.assertEqual(th.do_check(root), 0, "a volume counter must not gate")

    def test_check_fails_when_a_metric_status_changes(self):
        """A flip to warn, and above all to `unknown`, means a collector stopped
        measuring. Sensor rot must not be able to land silently."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            record = json.loads((root / th.LATEST).read_text())
            for m in record["metrics"]:
                if m["key"] == "known-failures":
                    m["status"] = th.UNKNOWN
            (root / th.LATEST).write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
            self.assertEqual(th.do_check(root), 1)

    def test_check_fails_when_the_tag_orphan_list_changes(self):
        """A new orphan means a build tag or make target just stranded a file.

        The committed snapshot is edited CONSISTENTLY -- list and count together,
        exactly as test_check_fails_when_an_rfc_gains_its_first_proof does for
        `rfc-unproven` -- so this drives the DRIFT path (a real event the tree and
        the snapshot disagree about) rather than the malformed-snapshot path,
        which test_check_fails_closed_on_a_count_that_contradicts_the_list owns.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            record = json.loads((root / th.LATEST).read_text())
            for m in record["metrics"]:
                if m["key"] == "tag-orphan":
                    m["orphans"] = [
                        {"file": "internal/ghost_test.go", "requires": "ze_x"}
                    ]
                    m["orphan_count"] = 1
            (root / th.LATEST).write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
            self.assertEqual(th.do_check(root), 1)

    def test_check_fails_closed_on_a_count_that_contradicts_the_list(self):
        """A snapshot whose orphan list and orphan count disagree is not
        measurement, so it must be refused rather than diffed.

        PREVENTS: the `tag-orphans` half of the vacuity. Without the cross-check,
        `orphans` could be `[]` while the counter said 8 and the gate would read
        it as "nothing is stranded" -- the goal state -- on both sides.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            record = json.loads((root / th.LATEST).read_text())
            for m in record["metrics"]:
                if m["key"] == "tag-orphan":
                    m["orphan_count"] = 8
            (root / th.LATEST).write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
            with self.assertRaises(th.CollectError) as ctx:
                th.do_check(root)
            self.assertIn("tag-orphan", str(ctx.exception))
            self.assertIn("have diverged", str(ctx.exception))

    def test_check_fails_closed_on_a_snapshot_without_the_orphan_field(self):
        """A snapshot predating the field must not read as "nothing is stranded"."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            record = json.loads((root / th.LATEST).read_text())
            for m in record["metrics"]:
                if m["key"] == "tag-orphan":
                    del m["orphans"]
            (root / th.LATEST).write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
            with self.assertRaises(th.CollectError) as ctx:
                th.do_check(root)
            self.assertIn("orphans", str(ctx.exception))
            self.assertIn("written before the field existed", str(ctx.exception))

    def test_check_fails_closed_on_a_status_outside_the_vocabulary(self):
        """A status no collector produces means the reader is on another field, so
        the gate must speak rather than quietly report drift."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            record = json.loads((root / th.LATEST).read_text())
            for m in record["metrics"]:
                if m["key"] == "known-failures":
                    m["status"] = "green"
            (root / th.LATEST).write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
            with self.assertRaises(th.CollectError) as ctx:
                th.do_check(root)
            self.assertIn("statuses", str(ctx.exception))

    def test_check_fails_when_an_rfc_gains_its_first_proof(self):
        """The event the fact exists to catch, driven through its entry point.

        The committed snapshot is edited to the state it would have AFTER rfc2
        earns a pair -- consistently, count and list together -- while the tree
        still says rfc2 is unproven. That divergence must fail the gate.
        PREVENTS: the fact reading a key nothing populates, which made this
        comparison `[] != []` and therefore always green.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            record = json.loads((root / th.LATEST).read_text())
            for m in record["metrics"]:
                if m["key"] == "rfc-unproven":
                    self.assertEqual(
                        m["unproven_rfcs"],
                        ["rfc2"],
                        "the generated snapshot must name the unproven RFC",
                    )
                    m["unproven_rfcs"] = []
                    m["unproven"] = th.ratio(0, 2)
            (root / th.LATEST).write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
            self.assertEqual(th.do_check(root), 1)

    def test_check_fails_closed_on_a_snapshot_without_the_field(self):
        """A snapshot written before the field existed must not read as `no RFCs
        are unproven`. It has no answer, and that is not the same as zero."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            record = json.loads((root / th.LATEST).read_text())
            for m in record["metrics"]:
                if m["key"] == "rfc-unproven":
                    del m["unproven_rfcs"]
            (root / th.LATEST).write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
            with self.assertRaises(th.CollectError) as ctx:
                th.do_check(root)
            self.assertIn("written before the field existed", str(ctx.exception))

    def test_check_fails_on_unparseable_latest_json(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            (root / th.LATEST).write_text("{not json}\n", encoding="utf-8")
            self.assertEqual(th.do_check(root), 1)

    def test_check_fails_when_the_page_is_deleted(self):
        """The page is not byte-gated, but it must exist."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            (root / th.PAGE).unlink()
            self.assertEqual(th.do_check(root), 1)

    def test_check_fails_when_latest_json_is_deleted(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            (root / th.LATEST).unlink()
            self.assertEqual(th.do_check(root), 1)

    def test_record_appends_exactly_one_row_and_refreshes_the_page(self):
        """AC-14."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            th.do_write(root)
            self.assertEqual(th.do_record(root), 0)
            rows = [
                json.loads(line)
                for line in (root / th.HISTORY).read_text().splitlines()
                if line.strip()
            ]
            self.assertEqual(len(rows), 1)
            for key in ("ts", "sha", "rfc_proof_numerator", "assert_nothing"):
                self.assertIn(key, rows[0])
            self.assertEqual(th.do_check(root), 0, "--record left the page stale")

    def test_record_refuses_before_dirtying_history(self):
        """A failing --record must not leave a sample behind for every retry."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / th.BASELINE).unlink()
            with self.assertRaises(th.CollectError):
                th.do_record(root)
            self.assertFalse(
                (root / th.HISTORY).exists(), "history was appended before the failure"
            )


class TestCrossProcessDeterminism(unittest.TestCase):
    """AC-2, for real.

    Rendering twice in ONE process proves nothing: CPython dict order is
    insertion-stable there by construction. Set iteration and any hash-derived
    ordering vary with PYTHONHASHSEED, which only differs BETWEEN processes.
    """

    def test_render_is_stable_across_hash_seeds(self):
        import subprocess as sp

        script = (
            "import sys; sys.path.insert(0, %r); import testing_health as th;"
            "ms=[th.Metric('b','Q2','second',th.WARN,'1',action='x',"
            "data={'worst':[{'k':2},{'j':3}],'buckets':{'2026':{'packages':1,"
            "'with_fuzz':0,'with_rfc_tag':0}}}),"
            "th.Metric('a','Q1','first',th.OK,'2',action='y'),"
            "th.Metric('c','Q3','third',th.UNKNOWN,'unknown',action='z')];"
            "sys.stdout.write(th.render_markdown(ms, {'history': []}))"
            % os.path.dirname(os.path.abspath(th.__file__))
        )
        outputs = []
        for seed in ("0", "1", "424242"):
            env = dict(os.environ, PYTHONHASHSEED=seed)
            res = sp.run(
                [sys.executable, "-c", script],
                capture_output=True,
                text=True,
                env=env,
                timeout=120,
            )
            self.assertEqual(res.returncode, 0, res.stderr)
            outputs.append(res.stdout)
        self.assertEqual(outputs[0], outputs[1], "render varies with PYTHONHASHSEED")
        self.assertEqual(outputs[1], outputs[2], "render varies with PYTHONHASHSEED")
        self.assertIn("first", outputs[0])


def git_init(root: Path) -> None:
    """Commit the fixture so the tracked-file collectors can see it."""
    import subprocess

    th._TRACKED_CACHE.clear()
    for args in (
        ["init", "-q"],
        ["config", "user.email", "t@example.com"],
        ["config", "user.name", "T"],
        ["add", "-A"],
        ["-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture"],
    ):
        subprocess.run(["git", "-C", str(root), *args], check=True, capture_output=True)


if __name__ == "__main__":
    unittest.main()
