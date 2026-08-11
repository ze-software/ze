#!/usr/bin/env python3
"""Unit tests for package_map.py (ai/PACKAGE-MAP.md generator)."""

from __future__ import annotations

import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from package_map import build, first_sentence, render


class TestPackageMap(unittest.TestCase):
    def test_responsibility_precedence(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "ai").mkdir()

            # 1. package doc wins.
            a = root / "internal" / "a"
            a.mkdir(parents=True)
            (a / "a.go").write_text("// Package a does the A thing.\npackage a\n")

            # 2. registry Description used when no package doc.
            b = root / "internal" / "b"
            b.mkdir(parents=True)
            (b / "register.go").write_text(
                "package b\nvar _ = registry.Registration{\n"
                '\tName: "beta",\n\tDescription: "does beta things",\n}\n'
            )

            # 3. neither -> TODO.
            c = root / "internal" / "c"
            c.mkdir(parents=True)
            (c / "c.go").write_text("package c\n\nfunc X() {}\n")

            # 4. pure embed package -> skipped.
            d = root / "internal" / "d" / "yang"
            d.mkdir(parents=True)
            (d / "embed.go").write_text("package yang\n")

            pkgs = build(root)
            self.assertEqual(pkgs["internal/a"], ("does the A thing", ""))
            self.assertEqual(pkgs["internal/b"], ("does beta things", "beta"))
            self.assertEqual(pkgs["internal/c"], ("TODO", ""))
            self.assertNotIn("internal/d/yang", pkgs)

    def test_multiline_package_doc_first_sentence(self):
        self.assertEqual(
            first_sentence("defines the family types (AFI, SAFI) and helpers"),
            "defines the family types (AFI, SAFI) and helpers",
        )
        self.assertEqual(
            first_sentence("does one thing. does another."), "does one thing"
        )

    def test_registered_name_from_a_package_constant(self):
        """A `Name:` written as a constant still reaches the map.

        VALIDATES: const_value resolves the three Go spellings of a same-file
        string constant, so a register.go that shares one name between its
        registration and a doctor check or filter keeps its plugin-name column.
        PREVENTS: the silent blank that a literal-only pattern produced. Seven
        register.go files name their registration through a constant, and the
        column exists to map a directory to that name.
        """
        cases = {
            "plain": ('const grName = "bgp-gr"\n', "bgp-gr"),
            "typed": ('const grName string = "bgp-gr"\n', "bgp-gr"),
            "block": ('const (\n\tother = 1\n\tgrName = "bgp-gr"\n)\n', "bgp-gr"),
            "elsewhere": ("", ""),
        }
        for label, (decl, want) in cases.items():
            with self.subTest(spelling=label), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / "ai").mkdir()
                pkg = root / "internal" / "g"
                pkg.mkdir(parents=True)
                (pkg / "register.go").write_text(
                    "package g\n" + decl + "var _ = registry.Registration{\n"
                    '\tName: grName,\n\tDescription: "does gr things",\n}\n'
                )
                self.assertEqual(build(root)["internal/g"], ("does gr things", want))

    def test_a_quoted_name_anywhere_beats_the_constant_lookup(self):
        """The constant is a fallback, never a competitor.

        VALIDATES: a register.go that declares a CLI command before its
        registration keeps the value it had. Preferring whichever `Name:`
        appears first would publish the command name as the plugin name, and
        emptied eight correct rows when it was tried against the real tree.
        PREVENTS: a blank-filling change silently rewriting rows it was not
        aimed at.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "ai").mkdir()
            pkg = root / "internal" / "l"
            pkg.mkdir(parents=True)
            (pkg / "register.go").write_text(
                'package l\nconst plugName = "unreached"\n'
                "var _ = command.Decl{\n"
                '\tName: "show l things",\n}\n'
                "var _ = registry.Registration{\n"
                '\tName: plugName,\n\tDescription: "does l things",\n}\n'
            )
            self.assertEqual(
                build(root)["internal/l"], ("does l things", "show l things")
            )

    def test_a_quoted_name_after_the_constant_still_beats_it(self):
        """The same rule with the two declarations the other way round.

        VALIDATES: quoted-anywhere-wins, when the constant `Name:` comes FIRST
        in the file. That is the order the real tree uses -- cos/register.go
        carries `Name: Name` in its registration and `Name: "show
        class-of-service"` in a command 150 lines later.
        PREVENTS: a first-occurrence-wins rule. The sibling test above cannot
        see that mutation, because its fixture puts the quoted name first, so
        first-wins picks the same value and the suite stays green. Measured
        against the real tree, first-wins rewrites 14 rows of ai/PACKAGE-MAP.md
        and empties 11 of them.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "ai").mkdir()
            pkg = root / "internal" / "c"
            pkg.mkdir(parents=True)
            (pkg / "register.go").write_text(
                'package c\nconst plugName = "cos"\n'
                "var _ = registry.Registration{\n"
                '\tName: plugName,\n\tDescription: "does c things",\n}\n'
                "var _ = command.Decl{\n"
                '\tName: "show class-of-service",\n}\n'
            )
            self.assertEqual(
                build(root)["internal/c"],
                ("does c things", "show class-of-service"),
            )

    def test_deterministic(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "ai").mkdir()
            p = root / "internal" / "p"
            p.mkdir(parents=True)
            (p / "p.go").write_text("// Package p provides P.\npackage p\n")
            self.assertEqual(render(build(root)), render(build(root)))


if __name__ == "__main__":
    unittest.main()
