#!/usr/bin/env python3
"""Tests for the website's testing-health renderer (../gh-pages/tools/render-test-health.py).

These live HERE, not in the gh-pages worktree, on purpose. Nothing in gh-pages
executes its `tools/test_*.py` files: `tools/build.py`, `update-website.sh` and <!-- doc-links: ignore (path in the sibling gh-pages worktree, not this repository) -->
gh-pages' own `.github/workflows/pages.yml` contain no test invocation, so <!-- doc-links: ignore (path in the sibling gh-pages worktree, not this repository) -->
`tools/test_render_doc.py` has never run. Adding a second unexecuted test file would have been an ironic way <!-- doc-links: ignore (path in the sibling gh-pages worktree, not this repository) -->
to ship a tool whose job is to find tests nothing runs.

Placed under scripts/dev/, TestPythonUnitTests (scripts/dev/python_tests_test.go)
globs and runs this on every `make ze-unit-test`. It skips cleanly when the
gh-pages worktree is not checked out beside the main repo.
"""

from __future__ import annotations

import json
import pathlib
import sys
import tempfile
import unittest
from importlib import machinery, util

MAIN = pathlib.Path(__file__).resolve().parents[2]
GH_PAGES_TOOLS = MAIN.parent / "gh-pages" / "tools"
RENDERER = GH_PAGES_TOOLS / "render-test-health.py"

HAVE_SITE = RENDERER.exists()


def load_renderer():
    """Load the hyphenated renderer by path, exactly as build.py's load_module does."""
    sys.path.insert(0, str(GH_PAGES_TOOLS))
    loader = machinery.SourceFileLoader("render_test_health", str(RENDERER))
    spec = util.spec_from_loader(loader.name, loader)
    module = util.module_from_spec(spec)
    loader.exec_module(module)
    return module


def metric(
    key="m", question="Q1", label="A metric", status="ok", value="1 / 2", **data
):
    out = {
        "key": key,
        "question": question,
        "label": label,
        "status": status,
        "value": value,
        "detail": "some detail",
        "action": "do the thing",
    }
    out.update(data)
    return out


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestRendererComputesNothing(unittest.TestCase):
    """The renderer invents no numbers of its own: it delegates to the ONE
    generator in the main repository (read-only, at build time) and falls back to
    the committed snapshot.

    VALIDATES: spec AC-16.
    PREVENTS: the site and the repository publishing two different answers to the
    same question, which is the defect the whole spec exists to correct.
    """

    def test_source_never_measures_the_tree_directly(self):
        """It may shell out to the canonical generator, but must not re-measure
        the tree itself (no rglob/glob, no test-file walking)."""
        src = RENDERER.read_text()
        for forbidden in ("rglob", "os.walk", "_test.go"):
            self.assertNotIn(
                forbidden,
                src,
                "the renderer must delegate to testing_health.py, never measure the tree itself",
            )

    def test_delegates_to_the_main_generator(self):
        """The freshness the user asked for comes from running the generator."""
        src = RENDERER.read_text()
        self.assertIn("--emit-page", src)
        self.assertIn("--json", src)
        self.assertIn("testing_health.py", src)

    def test_values_pass_through_verbatim(self):
        r = load_renderer()
        out = r.render_metric(metric(value="966 / 2716", status="warn"))
        self.assertIn("966 / 2716", out)


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestTrendHonesty(unittest.TestCase):
    """Below the sample threshold, say so rather than draw a line.

    VALIDATES: spec AC-15.
    PREVENTS: three points rendered as a confident direction.
    """

    def test_short_series_says_insufficient(self):
        r = load_renderer()
        out = r.render_trends([{"rfc_proof_percent": 10}, {"rfc_proof_percent": 20}])
        self.assertIn("insufficient data", out)
        self.assertNotIn("<polyline", out)

    def test_long_enough_series_draws_with_its_n(self):
        r = load_renderer()
        out = r.render_trends([{"rfc_proof_percent": v} for v in (10, 20, 30, 40, 50)])
        self.assertIn("<polyline", out)
        self.assertIn("<td>5</td>", out)

    def test_sparkline_declares_its_sample_count(self):
        r = load_renderer()
        self.assertIn("4 samples", r.sparkline([1, 2, 3, 4]))

    def test_sparkline_needs_two_points(self):
        r = load_renderer()
        self.assertEqual(r.sparkline([1]), "")

    def test_flat_series_does_not_divide_by_zero(self):
        r = load_renderer()
        self.assertIn("<polyline", r.sparkline([5, 5, 5]))


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestRatiosKeepTheirParts(unittest.TestCase):
    """A proportion bar is always accompanied by numerator and denominator.

    VALIDATES: spec AC-5.
    PREVENTS: a bar that looks healthy only because its denominator shrank.
    """

    def test_meter_prints_both_parts(self):
        r = load_renderer()
        out = r.meter(
            metric(proof_density={"numerator": 3, "denominator": 4, "percent": 75.0})
        )
        self.assertIn("3 of 4", out)
        self.assertIn("75.0%", out)

    def test_metric_without_a_ratio_gets_no_meter(self):
        r = load_renderer()
        self.assertEqual(r.meter(metric()), "")


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestAttentionOrdering(unittest.TestCase):
    """Unknown outranks warn outranks ok.

    VALIDATES: spec AC-6.
    PREVENTS: a dead collector rendering as green (sensor rot).
    """

    def test_unknown_sorts_above_warn(self):
        r = load_renderer()
        out = r.render_attention(
            [
                metric(key="a", label="warned", status="warn"),
                metric(key="b", label="unmeasured", status="unknown"),
            ]
        )
        self.assertLess(out.index("unmeasured"), out.index("warned"))

    def test_healthy_metrics_stay_out_of_the_attention_table(self):
        r = load_renderer()
        out = r.render_attention([metric(key="a", label="healthy", status="ok")])
        self.assertIn("Nothing outstanding", out)

    def test_status_order_is_total(self):
        r = load_renderer()
        self.assertLess(r.STATUS_ORDER["unknown"], r.STATUS_ORDER["warn"])
        self.assertLess(r.STATUS_ORDER["warn"], r.STATUS_ORDER["ok"])


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestMissingDataFailsLoudly(unittest.TestCase):
    """A health page with no health data must fail the site build, not render empty.

    These exercise the committed-file FALLBACK, so the generator is disabled: with
    the main tree present `load()` regenerates and never reads the file. The point
    is that a checkout where neither the generator nor the snapshot can be read
    warns and renders nothing, rather than rendering an empty page.
    """

    def test_missing_latest_json_warns(self):
        r = load_renderer()
        import sitelib

        before = len(sitelib.build_warnings())
        gen, latest = r.GENERATOR, r.LATEST
        try:
            r.GENERATOR = pathlib.Path("/nonexistent/testing_health.py")
            with tempfile.TemporaryDirectory() as tmp:
                r.LATEST = pathlib.Path(tmp) / "absent.json"
                metrics, _history = r.load()
            self.assertIsNone(metrics)
            self.assertGreater(len(sitelib.build_warnings()), before)
        finally:
            r.GENERATOR, r.LATEST = gen, latest

    def test_unreadable_latest_json_warns(self):
        r = load_renderer()
        import sitelib

        before = len(sitelib.build_warnings())
        gen, latest = r.GENERATOR, r.LATEST
        try:
            r.GENERATOR = pathlib.Path("/nonexistent/testing_health.py")
            with tempfile.TemporaryDirectory() as tmp:
                bad = pathlib.Path(tmp) / "latest.json"
                bad.write_text("{not json}")
                r.LATEST = bad
                metrics, _history = r.load()
            self.assertIsNone(metrics)
            self.assertGreater(len(sitelib.build_warnings()), before)
        finally:
            r.GENERATOR, r.LATEST = gen, latest

    def test_missing_generator_with_present_main_warns(self):
        """A present main tree whose generator file vanished must warn, not
        silently serve possibly-stale committed numbers -- the anti-stale point
        of running the generator at build time only holds if its absence is loud.
        """
        r = load_renderer()
        import sitelib

        self.assertTrue(r.MAIN.exists(), "test assumes the main tree is present")
        before = len(sitelib.build_warnings())
        gen = r.GENERATOR
        try:
            r.GENERATOR = pathlib.Path("/nonexistent/testing_health.py")
            self.assertIsNone(r._generate("--json"))
            self.assertGreater(len(sitelib.build_warnings()), before)
        finally:
            r.GENERATOR = gen


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestLoadRegeneratesFromTree(unittest.TestCase):
    """The published numbers come from the tree being built, not the commit.

    VALIDATES: the site regenerates test-health from main's live tree at build
    time, so `make build` in gh-pages publishes current numbers even when
    test/health/latest.json was committed before the last test change.
    PREVENTS: the site silently serving stale numbers (the reason this path
    exists) -- a regressing metric would keep reading green on the site until
    someone re-ran `make ze-test-health` and re-committed the snapshot.
    """

    def test_generator_output_supersedes_a_stale_snapshot(self):
        r = load_renderer()
        # A sentinel snapshot that could never come from the live generator.
        latest = r.LATEST
        try:
            with tempfile.TemporaryDirectory() as tmp:
                stale = pathlib.Path(tmp) / "latest.json"
                stale.write_text(
                    json.dumps(
                        {"metrics": [{"key": "STALE-SENTINEL", "value": "0 / 0"}]}
                    )
                )
                r.LATEST = stale
                metrics, _history = r.load()
            self.assertIsNotNone(metrics)
            keys = {m.get("key") for m in metrics}
            self.assertNotIn(
                "STALE-SENTINEL",
                keys,
                "load() served the committed snapshot instead of regenerating",
            )
            # And it really is the generator's live output.
            self.assertIn("rfc-proof-density", keys)
        finally:
            r.LATEST = latest


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestMarkdownSiblingMirrorsTheRepository(unittest.TestCase):
    """The site's index.md is a verbatim copy of the repository's page.

    VALIDATES: one Markdown document, not two.
    PREVENTS: the site composing its own summary of the same subject. It did
    exactly that -- an 18-line table here against the 193-line document in the
    main repository, already differing in wording and in which metrics each
    mentioned. Two documents about one subject drift by construction, which is
    the defect this whole feature exists to remove.
    """

    def test_sibling_is_the_generator_output_when_available(self):
        """The mirror is what the main generator emits, so the site is current."""
        r = load_renderer()
        fresh = r._generate("--emit-page")
        self.assertIsNotNone(fresh, "the main generator should run in this checkout")
        self.assertEqual(r.page_markdown(), fresh)

    def test_sibling_falls_back_to_committed_page_without_the_generator(self):
        r = load_renderer()
        original = r.GENERATOR
        try:
            r.GENERATOR = pathlib.Path("/nonexistent/testing_health.py")
            self.assertTrue(r.PAGE_MD.exists())
            self.assertEqual(r.page_markdown(), r.PAGE_MD.read_text())
        finally:
            r.GENERATOR = original

    def test_both_unavailable_warns_and_writes_nothing(self):
        r = load_renderer()
        import sitelib

        before = len(sitelib.build_warnings())
        gen, page = r.GENERATOR, r.PAGE_MD
        try:
            r.GENERATOR = pathlib.Path("/nonexistent/testing_health.py")
            with tempfile.TemporaryDirectory() as tmp:
                r.PAGE_MD = pathlib.Path(tmp) / "absent.md"
                self.assertIsNone(r.page_markdown())
            self.assertGreater(len(sitelib.build_warnings()), before)
        finally:
            r.GENERATOR, r.PAGE_MD = gen, page

    def test_renderer_composes_no_markdown_of_its_own(self):
        """A grep-level guard: reintroducing a local composer should fail here."""
        src = RENDERER.read_text()
        self.assertNotIn(
            "def render_markdown",
            src,
            "the site must mirror the repository's Markdown, never author its own",
        )

    def test_render_writes_both_the_html_and_the_markdown_sibling(self):
        """End-to-end: render() writes index.html AND the index.md mirror.

        VALIDATES: the composed on-disk write -- the one thing the unit tests of
        the constituent helpers cannot cover.
        PREVENTS: a regression that composes the page in memory but writes only
        one of the pair (or neither), shipping an HTML-only or empty health
        section. This is the coverage the old byte-verbatim mirror test carried
        before build-time regeneration made that exact assertion invalid.
        """
        import contextlib
        import io

        r = load_renderer()
        gen, dest = r.GENERATOR, r.DEST
        try:
            # Disable the generator so page_markdown falls back to the committed
            # page: no subprocess, so the test is fast and deterministic.
            r.GENERATOR = pathlib.Path("/nonexistent/testing_health.py")
            with tempfile.TemporaryDirectory() as tmp:
                r.DEST = pathlib.Path(tmp) / "quality" / "health" / "index.html"
                with contextlib.redirect_stdout(io.StringIO()):
                    r.render([metric(key="a", label="X", status="ok")], [])
                sibling = r.DEST.with_name("index.md")
                self.assertTrue(r.DEST.exists(), "index.html was not written")
                self.assertTrue(sibling.exists(), "index.md sibling was not written")
                self.assertGreater(len(r.DEST.read_text()), 0)
                self.assertGreater(len(sibling.read_text()), 0)
        finally:
            r.GENERATOR, r.DEST = gen, dest


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestEscaping(unittest.TestCase):
    """Every value reaching the HTML must be escaped.

    The values originate as file paths, test names, RFC ids and package names in
    another repository, so a crafted filename is a realistic delivery vector.
    The renderer had seven escaped interpolations and two unescaped ones on the
    same line; nothing tested any of them.
    """

    PAYLOAD = '"><script>alert(1)</script>'

    def assertNeutralised(self, out):
        self.assertNotIn("<script>", out, f"unescaped payload reached the HTML:\n{out}")

    def test_metric_value_is_escaped(self):
        r = load_renderer()
        self.assertNeutralised(r.render_metric(metric(value=self.PAYLOAD)))

    def test_metric_label_and_detail_are_escaped(self):
        r = load_renderer()
        self.assertNeutralised(r.render_metric(metric(label=self.PAYLOAD)))
        self.assertNeutralised(r.render_metric(metric(detail=self.PAYLOAD)))

    def test_meter_parts_are_escaped(self):
        r = load_renderer()
        out = r.meter(
            metric(
                proof_density={
                    "numerator": self.PAYLOAD,
                    "denominator": 4,
                    "percent": 75.0,
                }
            )
        )
        self.assertNeutralised(out)

    def test_attention_table_is_escaped(self):
        r = load_renderer()
        self.assertNeutralised(
            r.render_attention([metric(status="warn", label=self.PAYLOAD)])
        )

    def test_detail_table_cells_are_escaped(self):
        r = load_renderer()
        self.assertNeutralised(r.detail_table(metric(worst=[{"file": self.PAYLOAD}])))
        self.assertNeutralised(
            r.detail_table(metric(orphans=[{"file": self.PAYLOAD, "requires": "x"}]))
        )


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestMalformedMetricsDegrade(unittest.TestCase):
    """A malformed collector must not take the site build down with a traceback.

    PREVENTS: one bad metric costing the whole page, with a stack trace instead
    of a diagnosis.
    """

    def test_missing_fields_render(self):
        r = load_renderer()
        for bad in (
            {"question": "Q1"},
            {"question": "Q1", "status": None},
            {"question": "Q1", "status": "ok", "detail": None, "action": None},
            {"question": "Q1", "status": "typo-not-a-status", "value": "1"},
        ):
            with self.subTest(bad=bad):
                out = r.render_metric(bad)
                self.assertIn("th-card", out)

    def test_heterogeneous_worst_rows_render(self):
        r = load_renderer()
        out = r.detail_table(metric(worst=[{"a": 1}, {"b": 2}]))
        self.assertIn("<table", out)

    def test_non_dict_rows_are_skipped(self):
        r = load_renderer()
        self.assertIn("<table", r.detail_table(metric(worst=[{"a": 1}, "junk"])))

    def test_malformed_buckets_render(self):
        r = load_renderer()
        out = r.detail_table(metric(buckets={"2026": {"packages": 1}}))
        self.assertIn("<table", out)

    def test_unrecognised_status_sorts_with_unknown(self):
        """A typo'd status must not sort below ok and render as a calm card."""
        r = load_renderer()
        out = r.render_attention(
            [
                metric(key="a", label="warned", status="warn"),
                metric(key="b", label="typo", status="not-a-real-status"),
            ]
        )
        self.assertLess(out.index("typo"), out.index("warned"))

    def test_top_level_array_is_reported_not_raised(self):
        # Malformed committed snapshot, generator disabled so the fallback is read.
        r = load_renderer()
        import sitelib

        before = len(sitelib.build_warnings())
        gen, latest = r.GENERATOR, r.LATEST
        try:
            r.GENERATOR = pathlib.Path("/nonexistent/testing_health.py")
            with tempfile.TemporaryDirectory() as tmp:
                bad = pathlib.Path(tmp) / "latest.json"
                bad.write_text("[1, 2, 3]")
                r.LATEST = bad
                metrics, _history = r.load()
            self.assertIsNone(metrics)
            self.assertGreater(len(sitelib.build_warnings()), before)
        finally:
            r.GENERATOR, r.LATEST = gen, latest


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestSiteFactsSkipDirs(unittest.TestCase):
    """The published count must exclude third-party trees and nothing else.

    VALIDATES: spec AC-4.
    PREVENTS: the two errors this rule has already made. Too wide, and the
    headline counted 64,052 vendored dependency tests; matched at any depth, and
    it silently dropped first-party packages whose directory happened to be
    called `cache` or `gokrazy`.

    Placed here rather than in gh-pages because nothing in that worktree
    executes its tools/test_*.py files.
    """

    def _sitefacts(self):
        sys.path.insert(0, str(GH_PAGES_TOOLS))
        import sitefacts

        return sitefacts

    def test_module_cache_is_skipped(self):
        sf = self._sitefacts()
        root = pathlib.Path("/repo")
        self.assertTrue(
            sf.under_skip_dir(root / "gokrazy/modcache/dep/x_test.go", root)
        )

    def test_first_party_package_named_like_a_skip_is_kept(self):
        """`gokrazy` and `cache` are legitimate package names inside internal/."""
        sf = self._sitefacts()
        root = pathlib.Path("/repo")
        for keep in (
            "internal/component/gokrazy/gokrazy_test.go",
            "internal/component/resolve/cache/cache_test.go",
            "internal/component/bgp/plugins/cmd/cache/cache_test.go",
        ):
            self.assertFalse(
                sf.under_skip_dir(root / keep, root),
                f"{keep} is first-party and must be counted",
            )

    def test_nested_vendor_is_skipped_at_any_depth(self):
        sf = self._sitefacts()
        root = pathlib.Path("/repo")
        self.assertTrue(sf.under_skip_dir(root / "internal/x/vendor/y_test.go", root))

    def test_no_tracked_first_party_test_is_dropped(self):
        """Drive it from the real repository, not from hand-picked paths."""
        sf = self._sitefacts()
        import subprocess

        out = subprocess.run(
            ["git", "-C", str(MAIN), "ls-files", "*_test.go"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout
        dropped = [
            name
            for name in out.splitlines()
            if sf.under_skip_dir(MAIN / name, MAIN) and not name.startswith("gokrazy/")
        ]
        self.assertEqual(dropped, [], f"first-party tests wrongly excluded: {dropped}")


@unittest.skipUnless(HAVE_SITE, "gh-pages worktree not checked out beside main/")
class TestThresholdsAgreeAcrossTheBoundary(unittest.TestCase):
    """The two sides must not disagree about when a trend is drawable.

    PREVENTS: the repository page saying "insufficient data" while the site
    draws a line from the same history, in a design whose premise is one
    counter and one answer.
    """

    def test_min_samples_matches(self):
        r = load_renderer()
        sys.path.insert(0, str(MAIN / "scripts" / "dev"))
        import testing_health as th

        self.assertEqual(
            r.MIN_SAMPLES,
            th.MIN_SAMPLES,
            "MIN_SAMPLES differs between the page generator and the site renderer",
        )


if __name__ == "__main__":
    unittest.main()
