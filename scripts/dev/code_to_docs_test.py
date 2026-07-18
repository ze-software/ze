#!/usr/bin/env python3
"""Unit tests for code_to_docs.py (ai/CODE-TO-DOCS.md generator)."""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from code_to_docs import extract_paths, filter_gitignored


class TestExtractPaths(unittest.TestCase):
    def test_semicolon_separated(self):
        self.assertEqual(
            extract_paths("internal/a.go -- one; cmd/b.go -- two"),
            ["internal/a.go", "cmd/b.go"],
        )

    def test_comma_relative_paths(self):
        # A bare filename after a full path inherits that path's directory.
        self.assertEqual(
            extract_paths("internal/component/x/a.go, b.go"),
            ["internal/component/x/a.go", "internal/component/x/b.go"],
        )


class TestFilterGitignored(unittest.TestCase):
    def _git(self, root: Path, *args: str) -> None:
        subprocess.run(
            ["git", "-C", str(root), *args],
            check=True,
            capture_output=True,
            text=True,
        )

    def test_skips_gitignored_docs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._git(root, "init")
            (root / ".gitignore").write_text("docs/research/comparison/\n")
            (root / "docs").mkdir()
            (root / "docs" / "keep.md").write_text("# keep\n")
            research = root / "docs" / "research" / "comparison" / "freertr"
            research.mkdir(parents=True)
            (research / "23-concurrent-editing.md").write_text("# ignored\n")

            paths = sorted((root / "docs").rglob("*.md"))
            kept = [str(p.relative_to(root)) for p in filter_gitignored(root, paths)]

            self.assertIn("docs/keep.md", kept)
            self.assertNotIn(
                "docs/research/comparison/freertr/23-concurrent-editing.md", kept
            )

    def test_no_git_repo_falls_back(self):
        # Outside a git repository, check-ignore errors (128); index everything.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "a.md").write_text("# a\n")
            paths = sorted((root / "docs").rglob("*.md"))
            self.assertEqual(filter_gitignored(root, paths), paths)

    def test_empty(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(filter_gitignored(Path(tmp), []), [])


if __name__ == "__main__":
    unittest.main()
