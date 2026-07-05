#!/usr/bin/env python3
"""Unit tests for docs_to_code.py (ai/DOCS-TO-CODE.md generator)."""

from __future__ import annotations

import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from docs_to_code import build, render


class TestDocsToCode(unittest.TestCase):
    def _tree(self, tmp: str) -> Path:
        root = Path(tmp)
        (root / "ai").mkdir()
        (root / "internal" / "foo").mkdir(parents=True)
        return root

    def test_inverts_design_headers(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._tree(tmp)
            (root / "internal" / "foo" / "a.go").write_text(
                "// Design: docs/architecture/x.md — topic A\npackage foo\n"
            )
            (root / "internal" / "foo" / "b.go").write_text(
                "// Design: docs/architecture/x.md -- topic B\npackage foo\n"
            )
            index = build(root)
            self.assertEqual(
                index["docs/architecture/x.md"],
                {("internal/foo/a.go", "topic A"), ("internal/foo/b.go", "topic B")},
            )

    def test_skips_none_and_vendor(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._tree(tmp)
            (root / "internal" / "foo" / "c.go").write_text(
                "// Design: (none — predates documentation)\npackage foo\n"
            )
            (root / "vendor" / "z").mkdir(parents=True)
            (root / "vendor" / "z" / "z.go").write_text(
                "// Design: docs/architecture/y.md — vendored\npackage z\n"
            )
            index = build(root)
            # `(none ...)` is not a doc path; vendored files are skipped.
            self.assertEqual(index, {})

    def test_deterministic(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._tree(tmp)
            (root / "internal" / "foo" / "a.go").write_text(
                "// Design: docs/a.md — one\npackage foo\n"
            )
            (root / "internal" / "foo" / "b.go").write_text(
                "// Design: docs/a.md — two\npackage foo\n"
            )
            self.assertEqual(render(build(root)), render(build(root)))


if __name__ == "__main__":
    unittest.main()
