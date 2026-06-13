#!/usr/bin/env python3
"""Unit tests for validate.py check functions."""

from __future__ import annotations

import os
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from validate import (
    Finding,
    check_cli_handler_coverage,
    check_cross_package_wiring,
    check_source_anchor_line_numbers,
    check_source_anchor_stale_paths,
    check_spec_ac_completeness,
)


class TestCleanPasses(unittest.TestCase):
    """AC-6: Clean codebase produces no findings."""

    def test_no_findings_on_clean_tree(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "internal" / "foo").mkdir(parents=True)
            (root / "plan").mkdir()

            (root / "internal" / "foo" / "bar.go").write_text(
                "package foo\n\nfunc internal() {}\n"
            )
            (root / "docs" / "test.md").write_text(
                "<!-- source: internal/foo/bar.go -- Foo package -->\n"
            )

            findings = []
            findings.extend(check_source_anchor_line_numbers(root))
            findings.extend(check_source_anchor_stale_paths(root))
            findings.extend(check_spec_ac_completeness(root))
            self.assertEqual(findings, [], f"unexpected findings: {findings}")


class TestLineNumberAnchor(unittest.TestCase):
    """AC-1: Source anchor with line number *.go:47 reports ISSUE."""

    def test_detects_line_number_anchor(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "test.md").write_text(
                "<!-- source: internal/foo/bar.go:47 -- some function -->\n"
            )
            findings = check_source_anchor_line_numbers(root)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0].severity, "ISSUE")
            self.assertIn(":47", findings[0].message)

    def test_ignores_anchor_without_line_number(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "test.md").write_text(
                "<!-- source: internal/foo/bar.go -- some function -->\n"
            )
            findings = check_source_anchor_line_numbers(root)
            self.assertEqual(findings, [])


class TestStaleAnchorPath(unittest.TestCase):
    """AC-2: Source anchor pointing to non-existent file reports ISSUE."""

    def test_detects_missing_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "test.md").write_text(
                "<!-- source: internal/nonexistent/file.go -- gone -->\n"
            )
            findings = check_source_anchor_stale_paths(root)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0].severity, "ISSUE")
            self.assertIn("non-existent", findings[0].message)

    def test_passes_for_existing_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "internal" / "foo").mkdir(parents=True)
            (root / "internal" / "foo" / "bar.go").write_text("package foo\n")
            (root / "docs" / "test.md").write_text(
                "<!-- source: internal/foo/bar.go -- Foo -->\n"
            )
            findings = check_source_anchor_stale_paths(root)
            self.assertEqual(findings, [])

    def test_skips_url_anchors(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "test.md").write_text(
                "<!-- source: https://example.com/repo -- upstream -->\n"
            )
            findings = check_source_anchor_stale_paths(root)
            self.assertEqual(findings, [])


class TestCrossPackageWiring(unittest.TestCase):
    """AC-3: Exported symbol with no cross-package non-test caller."""

    def test_detects_same_package_only_export(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            pkg2 = root / "internal" / "beta"
            pkg2.mkdir(parents=True)

            (pkg / "handler.go").write_text("package alpha\n\nfunc UnusedExport() {}\n")
            (pkg / "handler_test.go").write_text(
                'package alpha\n\nimport "testing"\n\n'
                "func TestIt(t *testing.T) { UnusedExport() }\n"
            )
            (pkg2 / "other.go").write_text("package beta\n\nfunc Other() {}\n")

            changed = ["internal/alpha/handler.go"]
            findings = check_cross_package_wiring(root, changed)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0].severity, "ISSUE")
            self.assertIn("UnusedExport", findings[0].message)

    def test_passes_for_cross_package_caller(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            pkg2 = root / "internal" / "beta"
            pkg2.mkdir(parents=True)

            (pkg / "handler.go").write_text("package alpha\n\nfunc UsedExport() {}\n")
            (pkg2 / "consumer.go").write_text(
                'package beta\n\nimport "alpha"\n\nfunc Use() { alpha.UsedExport() }\n'
            )

            changed = ["internal/alpha/handler.go"]
            findings = check_cross_package_wiring(root, changed)
            self.assertEqual(findings, [])


class TestSpecACCompleteness(unittest.TestCase):
    """AC-4: Spec AC row with empty 'Demonstrated By' column."""

    def test_detects_empty_demonstrated_by(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-test-feature.md").write_text(
                textwrap.dedent("""\
                | Field | Value |
                |-------|-------|
                | Status | in-progress |

                ## Implementation Audit

                ### Acceptance Criteria
                | AC ID | Status | Demonstrated By | Notes |
                |-------|--------|-----------------|-------|
                | AC-1 | done | test_foo | works |
                | AC-2 |  |  |  |
                """)
            )
            findings = check_spec_ac_completeness(root)
            self.assertEqual(len(findings), 1)
            self.assertIn("AC-2", findings[0].message)

    def test_passes_when_all_filled(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-test-feature.md").write_text(
                textwrap.dedent("""\
                | Field | Value |
                |-------|-------|
                | Status | in-progress |

                ## Implementation Audit

                ### Acceptance Criteria
                | AC ID | Status | Demonstrated By | Notes |
                |-------|--------|-----------------|-------|
                | AC-1 | done | test_foo | works |
                """)
            )
            findings = check_spec_ac_completeness(root)
            self.assertEqual(findings, [])

    def test_skips_design_status_specs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-design-only.md").write_text(
                textwrap.dedent("""\
                | Field | Value |
                |-------|-------|
                | Status | design |

                ## Implementation Audit

                ### Acceptance Criteria
                | AC ID | Status | Demonstrated By | Notes |
                |-------|--------|-----------------|-------|
                | AC-1 |  |  |  |
                """)
            )
            findings = check_spec_ac_completeness(root)
            self.assertEqual(findings, [])


if __name__ == "__main__":
    unittest.main()
