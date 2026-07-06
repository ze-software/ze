#!/usr/bin/env python3
"""Unit tests for discovery_sources.py (shared discovery-index source predicate)."""

from __future__ import annotations

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))
from discovery_sources import GENERATORS, OUTPUTS, is_discovery_source


class TestDiscoverySources(unittest.TestCase):
    def test_generators_and_outputs(self):
        for p in GENERATORS + OUTPUTS:
            self.assertTrue(is_discovery_source(p), p)

    def test_makefile_and_mk(self):
        self.assertTrue(is_discovery_source("Makefile"))
        self.assertTrue(is_discovery_source("mk/inventory.mk"))

    def test_learned_summary(self):
        self.assertTrue(is_discovery_source("plan/learned/1067-topic.md"))
        self.assertFalse(is_discovery_source("plan/spec-topic.md"))

    def test_register_go(self):
        self.assertTrue(is_discovery_source("internal/x/register.go"))

    def test_go_header_markers(self):
        self.assertTrue(
            is_discovery_source(
                "internal/x/y.go", "// Package y does things\npackage y\n"
            )
        )
        self.assertTrue(
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


if __name__ == "__main__":
    unittest.main()
