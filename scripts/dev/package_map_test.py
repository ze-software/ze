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
                'package b\nvar _ = registry.Registration{\n'
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
        self.assertEqual(first_sentence("does one thing. does another."), "does one thing")

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
