#!/usr/bin/env python3
"""Unit tests for discovery_sources.py (shared discovery-index source predicate)."""

from __future__ import annotations

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))
from discovery_sources import (
    GENERATORS,
    OUTPUTS,
    PACKAGE_MAP,
    indexes_fed_by,
    is_discovery_source,
)


class TestDiscoverySources(unittest.TestCase):
    def test_generators_and_outputs(self):
        for p in GENERATORS + OUTPUTS:
            self.assertTrue(is_discovery_source(p), p)

    def test_makefile_and_mk(self):
        self.assertTrue(is_discovery_source("Makefile"))
        self.assertTrue(is_discovery_source("mk/check-rules.mk"))

    def test_learned_summary_not_a_source(self):
        self.assertFalse(is_discovery_source("plan/learned/1067-topic.md"))
        self.assertFalse(is_discovery_source("plan/spec-topic.md"))

    def test_register_go(self):
        self.assertTrue(is_discovery_source("internal/x/register.go"))

    def test_go_header_markers(self):
        self.assertTrue(
            is_discovery_source(
                "internal/x/y.go", "// Package y does things\npackage y\n"
            )
        )
        # A `// Design:` header feeds ai/DOCS-TO-CODE.md, which is no longer
        # tracked (.gitignore), so it obliges no commit to refresh anything.
        self.assertFalse(
            is_discovery_source(
                "internal/x/y.go", "// Design: docs/architecture/y.md\npackage y\n"
            )
        )
        self.assertFalse(
            is_discovery_source("internal/x/y.go", "package y\n\nfunc F() {}\n")
        )

    def test_test_go_and_unrelated_excluded(self):
        self.assertFalse(is_discovery_source("internal/x/y_test.go", "// Package y\n"))
        self.assertFalse(is_discovery_source("docs/architecture/core-design.md"))
        self.assertFalse(is_discovery_source("ai/digests/bgp-reactor.md"))


class TestIndexesFedBy(unittest.TestCase):
    """The per-index source map (T-6): a source feeds SPECIFIC indexes, not "some
    index", so the commit gate can demand only the indexes a commit's sources
    actually feed."""

    def test_learned_summary_feeds_nothing(self):
        self.assertEqual(indexes_fed_by("plan/learned/1067-topic.md"), frozenset())

    def test_design_header_feeds_nothing_since_docs_to_code_is_untracked(self):
        """ai/DOCS-TO-CODE.md is generated on demand and gitignored.

        The gate asks "must this commit refresh a COMMITTED index". A file git
        does not hold can never be the answer, so a `// Design:` header is no
        longer a discovery source at all.
        """
        self.assertEqual(
            indexes_fed_by("internal/x/y.go", "// Design: docs/x.md\npackage y\n"),
            frozenset(),
        )

    def test_package_header_feeds_only_package_map(self):
        self.assertEqual(
            indexes_fed_by("internal/x/y.go", "// Package y does things\npackage y\n"),
            frozenset({PACKAGE_MAP}),
        )

    def test_register_go_feeds_only_package_map(self):
        self.assertEqual(
            indexes_fed_by("internal/x/register.go"), frozenset({PACKAGE_MAP})
        )

    def test_generator_feeds_only_its_output(self):
        for gen, out in zip(GENERATORS, OUTPUTS):
            self.assertEqual(indexes_fed_by(gen), frozenset({out}), gen)

    def test_makefile_and_mk_feed_all(self):
        # The ze-discovery-index-update wiring runs every generator; a change there can
        # drift any index, so demand all (conservative).
        self.assertEqual(indexes_fed_by("Makefile"), frozenset(OUTPUTS))
        self.assertEqual(indexes_fed_by("mk/check-rules.mk"), frozenset(OUTPUTS))

    def test_committed_index_feeds_only_itself(self):
        self.assertEqual(indexes_fed_by(PACKAGE_MAP), frozenset({PACKAGE_MAP}))

    def test_unrelated_feeds_nothing(self):
        self.assertEqual(indexes_fed_by("docs/guide/x.md"), frozenset())
        self.assertEqual(
            indexes_fed_by("internal/x/y.go", "package y\n\nfunc F() {}\n"),
            frozenset(),
        )
        self.assertEqual(
            indexes_fed_by("internal/x/y_test.go", "// Package y\n"), frozenset()
        )


if __name__ == "__main__":
    unittest.main()
