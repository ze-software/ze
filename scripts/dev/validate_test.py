#!/usr/bin/env python3
"""Unit tests for validate.py check functions."""

from __future__ import annotations

import os
import sys
import tempfile
import textwrap
import unittest
from unittest import mock
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from validate import (
    check_cross_package_wiring,
    check_source_anchor_line_numbers,
    check_source_anchor_stale_paths,
    check_spec_ac_completeness,
    main,
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

    def test_detects_missing_file_outside_the_code_to_docs_roots(self):
        # This check resolves ANY repo-relative anchor. The gated walk in
        # code_to_docs.py (extract_paths) keeps only paths under nine
        # PATH_PREFIX roots, so 74 anchors in docs/ -- under tools/, gokrazy/,
        # ai/, demos/, .github/ and docs/ itself -- are seen by this check
        # alone. It reaches the gate through `make ze-validate-tree`, a stage of
        # both verify modes (stagesForMode, scripts/status/verify_run.go).
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "test.md").write_text(
                "<!-- source: tools/gone/helper.py -- retired -->\n"
            )
            findings = check_source_anchor_stale_paths(root)
            self.assertEqual(len(findings), 1)
            self.assertIn("tools/gone/helper.py", findings[0].message)

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

    def test_skips_external_home_relative_anchors(self):
        # Provenance for reverse-engineered external code lives outside the
        # repo (e.g. a local checkout under ~/Code/...); it cannot be resolved
        # here and must not be flagged as a stale in-repo path.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "test.md").write_text(
                "<!-- source: ~/Code/github.com/Vendor/agent/agent.py -- API -->\n"
            )
            findings = check_source_anchor_stale_paths(root)
            self.assertEqual(findings, [])

    def test_skips_external_absolute_anchors(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "test.md").write_text(
                "<!-- source: /opt/vendor/agent/agent.py -- API -->\n"
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

    def test_type_wired_through_its_constants(self):
        """Typed enum whose bare name never appears cross-package is still wired
        when callers reference its constants (the RouteVerb pattern)."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            pkg2 = root / "internal" / "beta"
            pkg2.mkdir(parents=True)

            (pkg / "verb.go").write_text(
                "package alpha\n\n"
                "type Verb uint8\n\n"
                "const (\n"
                "\tVerbSkip Verb = iota\n"
                "\tVerbInstall\n"
                "\tVerbRemove\n"
                ")\n"
            )
            # Consumer switches on the constants and never spells the type name.
            (pkg2 / "consumer.go").write_text(
                'package beta\n\nimport "alpha"\n\n'
                "func Use(v int) {\n"
                "\tswitch v {\n"
                "\tcase int(alpha.VerbInstall):\n"
                "\tcase int(alpha.VerbRemove):\n"
                "\t}\n"
                "}\n"
            )

            changed = ["internal/alpha/verb.go"]
            findings = check_cross_package_wiring(root, changed)
            self.assertEqual(findings, [], f"unexpected findings: {findings}")

    def test_type_with_unused_constants_still_flagged(self):
        """A typed enum whose constants are also unused stays flagged."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)

            (pkg / "verb.go").write_text(
                "package alpha\n\n"
                "type Verb uint8\n\n"
                "const (\n"
                "\tVerbSkip Verb = iota\n"
                "\tVerbInstall\n"
                ")\n"
            )

            changed = ["internal/alpha/verb.go"]
            findings = check_cross_package_wiring(root, changed)
            self.assertEqual(len(findings), 1)
            self.assertIn("Verb", findings[0].message)

    def _wiring_with_verb_decl(self, decl: str, consumer_const: str):
        """Helper: a Verb type declared via `decl`, with a cross-package
        consumer referencing `consumer_const`. Returns the findings."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            pkg2 = root / "internal" / "beta"
            pkg2.mkdir(parents=True)

            (pkg / "verb.go").write_text(f"package alpha\n\n{decl}\n")
            (pkg2 / "consumer.go").write_text(
                'package beta\n\nimport "alpha"\n\n'
                f"func Use() {{ _ = alpha.{consumer_const} }}\n"
            )
            return check_cross_package_wiring(root, ["internal/alpha/verb.go"])

    def test_type_wired_via_single_line_const(self):
        """A type whose constants are declared with a single-line (non-block)
        const is still recognized as wired (NOTE 1)."""
        decl = "type Verb uint8\n\nconst Solo Verb = 3\n"
        self.assertEqual(self._wiring_with_verb_decl(decl, "Solo"), [])

    def test_type_wired_via_multi_name_const(self):
        """A multi-name const spec (`A, B Verb = ...`) wires the type (NOTE 1)."""
        decl = "type Verb uint8\n\nconst (\n\tVerbA, VerbB Verb = 1, 2\n)\n"
        self.assertEqual(self._wiring_with_verb_decl(decl, "VerbB"), [])

    def test_type_wired_via_block_with_trailing_comment(self):
        """A const block opened with a trailing comment still parses (NOTE 1)."""
        decl = (
            "type Verb uint8\n\n"
            "const ( // direction codes\n"
            "\tVerbSkip Verb = iota\n"
            "\tVerbGo\n"
            ")\n"
        )
        self.assertEqual(self._wiring_with_verb_decl(decl, "VerbGo"), [])

    def test_type_wired_via_constructor_return(self):
        """A type reached only through an exported constructor/accessor that
        returns it (NewEvaluator() *Evaluator, Global() *Evaluator) is wired:
        cross-package callers use `:=` and never spell the type name."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            pkg2 = root / "internal" / "beta"
            pkg2.mkdir(parents=True)

            (pkg / "eval.go").write_text(
                "package alpha\n\n"
                "type Evaluator struct{}\n\n"
                "func NewEvaluator() *Evaluator { return &Evaluator{} }\n"
            )
            # Consumer builds one via the constructor and never spells the type.
            (pkg2 / "consumer.go").write_text(
                'package beta\n\nimport "alpha"\n\n'
                "func Use() { ev := alpha.NewEvaluator(); _ = ev }\n"
            )

            findings = check_cross_package_wiring(root, ["internal/alpha/eval.go"])
            self.assertEqual(findings, [], f"unexpected findings: {findings}")

    def test_type_returned_by_unused_constructor_still_flagged(self):
        """The constructor-seam leniency must not over-suppress: a type whose
        only constructor itself has no cross-package caller stays flagged."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)

            (pkg / "eval.go").write_text(
                "package alpha\n\n"
                "type Evaluator struct{}\n\n"
                "func NewEvaluator() *Evaluator { return &Evaluator{} }\n"
            )

            findings = check_cross_package_wiring(root, ["internal/alpha/eval.go"])
            self.assertTrue(
                any(
                    f.message
                    == "exported symbol Evaluator has no cross-package non-test caller"
                    for f in findings
                ),
                f"Evaluator should stay flagged; got: {findings}",
            )

    def test_type_wired_as_param_of_wired_setter(self):
        """A type reached only as the PARAMETER of an exported setter that is
        itself wired (SetPingFactory(PingFactory)) is wired: Go's structural
        assignability means the caller passes a plain func literal or a concrete
        value and never spells the parameter type."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            pkg2 = root / "internal" / "beta"
            pkg2.mkdir(parents=True)

            # Factory is never named outside alpha; the field holding it is
            # unexported, so the struct-field seam cannot cover it either.
            (pkg / "model.go").write_text(
                "package alpha\n\n"
                "type Factory func(x int) error\n\n"
                "type Model struct {\n\tf Factory\n}\n\n"
                "func NewModel() *Model { return &Model{} }\n\n"
                "func (m *Model) SetFactory(f Factory) { m.f = f }\n"
            )
            # Consumer passes a bare func literal: assignability, no type name.
            (pkg2 / "consumer.go").write_text(
                'package beta\n\nimport "alpha"\n\n'
                "func Use() {\n"
                "\tm := alpha.NewModel()\n"
                "\tm.SetFactory(func(x int) error { return nil })\n"
                "}\n"
            )

            findings = check_cross_package_wiring(root, ["internal/alpha/model.go"])
            self.assertEqual(findings, [], f"unexpected findings: {findings}")

    def test_type_param_of_unwired_func_still_flagged(self):
        """The parameter-seam leniency must not over-suppress: a type that is
        only the parameter of an exported func with NO cross-package caller
        stays flagged. Without this bound the exemption would clear nearly every
        dead exported type, since most appear in some signature."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)

            (pkg / "model.go").write_text(
                "package alpha\n\n"
                "type Factory func(x int) error\n\n"
                "func SetFactory(f Factory) { _ = f }\n"
            )

            findings = check_cross_package_wiring(root, ["internal/alpha/model.go"])
            self.assertTrue(
                any(
                    f.message
                    == "exported symbol Factory has no cross-package non-test caller"
                    for f in findings
                ),
                f"Factory should stay flagged; got: {findings}",
            )

    def test_value_only_spec_does_not_inherit_type(self):
        """A value-only spec (`X = expr`, no type) after a typed spec must NOT
        be attributed to the enum type, matching Go's iota rules (NOTE 1)."""
        # YOnly has an explicit value and no type, so it is untyped int, not Verb.
        # Referencing only YOnly cross-package must NOT clear the Verb finding.
        decl = "type Verb uint8\n\nconst (\n\tVerbX Verb = iota\n\tYOnly = 99\n)\n"
        findings = self._wiring_with_verb_decl(decl, "YOnly")
        self.assertEqual(len(findings), 1)
        self.assertIn("Verb", findings[0].message)

    def test_type_wired_as_struct_field(self):
        """A type reached only through a struct field (serialized/wire structs)
        is recognized as wired, like the constants case (NOTE 3)."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            pkg2 = root / "internal" / "beta"
            pkg2.mkdir(parents=True)

            # Sub is never named outside alpha; it is only the type of Top.S.
            (pkg / "model.go").write_text(
                "package alpha\n\n"
                "type Sub uint8\n\n"
                "type Top struct {\n"
                "\tS    Sub\n"
                "\tList []Sub\n"
                "}\n"
            )
            (pkg2 / "consumer.go").write_text(
                'package beta\n\nimport "alpha"\n\nfunc Use(t alpha.Top) { _ = t.S }\n'
            )

            findings = check_cross_package_wiring(root, ["internal/alpha/model.go"])
            self.assertEqual(findings, [], f"unexpected findings: {findings}")

    def test_plain_unused_type_still_flagged(self):
        """A struct type with no cross-package caller and not used as a field is
        still flagged -- the field-type leniency must not over-suppress."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            (pkg / "model.go").write_text(
                "package alpha\n\ntype Orphan struct {\n\tX int\n}\n"
            )
            findings = check_cross_package_wiring(root, ["internal/alpha/model.go"])
            self.assertEqual(len(findings), 1)
            self.assertIn("Orphan", findings[0].message)

    def test_fortest_helper_is_exempt(self):
        """An exported *ForTest helper is test-only by convention and is exempt
        from the production-wiring check (NOTE 3)."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "alpha"
            pkg.mkdir(parents=True)
            (pkg / "reset.go").write_text(
                "package alpha\n\nfunc ResetStateForTest() {}\n"
            )
            findings = check_cross_package_wiring(root, ["internal/alpha/reset.go"])
            self.assertEqual(findings, [], f"unexpected findings: {findings}")


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


class TestChangedFileSelection(unittest.TestCase):
    """`make ze-validate-tree` declares an EMPTY changed set.

    The Makefile recipe is `validate.py --root . --changed-file ''`, and the
    stage runs inside both `make ze-verify` modes (stagesForMode,
    scripts/status/verify_run.go). Which checks run must be decided by the flag
    being GIVEN. An empty list is falsy, so a selection written on truthiness
    reads the empty set as "no flag", falls back to git diff, and puts
    check_cross_package_wiring and check_cli_handler_coverage back inside the
    gate -- where, in a shared checkout, they judge other sessions'
    half-written files. Nothing else pins main(): the Go test can only see the
    string the recipe passes.
    """

    def _repo(self, root: Path) -> None:
        (root / "go.mod").write_text("module example.com/x\n")
        (root / "docs").mkdir()
        (root / "plan").mkdir()
        pkg = root / "internal" / "alpha"
        pkg.mkdir(parents=True)
        (pkg / "handler.go").write_text("package alpha\n\nfunc UnusedExport() {}\n")

    def test_empty_changed_set_runs_neither_changed_file_check(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._repo(root)

            # The fixture discriminates: the same file IS a finding when the
            # changed set holds it, so a green main() means the check was
            # skipped rather than satisfied.
            self.assertEqual(
                len(check_cross_package_wiring(root, ["internal/alpha/handler.go"])), 1
            )

            argv = ["validate.py", "--root", str(root), "--changed-file", ""]
            with mock.patch.object(sys, "argv", argv):
                self.assertEqual(main(), 0)

    def test_a_named_changed_file_still_reaches_the_changed_file_checks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._repo(root)

            argv = [
                "validate.py",
                "--root",
                str(root),
                "--changed-file",
                "internal/alpha/handler.go",
            ]
            with mock.patch.object(sys, "argv", argv):
                self.assertEqual(main(), 1)


if __name__ == "__main__":
    unittest.main()
