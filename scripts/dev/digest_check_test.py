#!/usr/bin/env python3
"""Unit tests for digest_check.py (ai/digests/*.md anchor validator)."""

from __future__ import annotations

import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from digest_check import anchors_in, check_digest, parse_bases, resolve


def write(root: Path, rel: str, lines: int) -> None:
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text("\n".join(f"line {i}" for i in range(1, lines + 1)) + "\n")


def digest(root: Path, name: str, body: str) -> Path:
    d = root / "ai" / "digests"
    d.mkdir(parents=True, exist_ok=True)
    p = d / name
    p.write_text(body)
    return p


class TestParsing(unittest.TestCase):
    def test_parse_bases_multi(self):
        text = "<!-- digest-base: internal/a  internal/b, pkg/c -->\n"
        self.assertEqual(parse_bases(text), ["internal/a", "internal/b", "pkg/c"])

    def test_anchors_only_backticked_code_tokens(self):
        text = "flow `peer.go:12` and `attribute/wire.go:3-9` but not `EventBus` or `s.mu.Lock()` or `ze.bgp.x`"
        got = anchors_in(text)
        self.assertEqual(got, [("peer.go", 12, None), ("attribute/wire.go", 3, 9)])

    def test_comma_list_expands_to_multiple_anchors(self):
        # `file.go:12,20-24` is two cites; both must be validated, not dropped.
        self.assertEqual(
            anchors_in("see `cmd.go:53,72` and `x.go:12,20-24`"),
            [
                ("cmd.go", 53, None),
                ("cmd.go", 72, None),
                ("x.go", 12, None),
                ("x.go", 20, 24),
            ],
        )


class TestResolve(unittest.TestCase):
    def test_repo_relative_short_circuits(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.assertEqual(resolve(root, [], "internal/x/y.go"), ["internal/x/y.go"])

    def test_cross_base_ambiguity_fails_closed(self):
        # Same basename under two different bases -> ambiguous (both returned),
        # regardless of declared order. It must NOT silently pick the first base.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/a/cmd.go", 5)
            write(root, "internal/b/cmd.go", 5)
            self.assertEqual(
                sorted(resolve(root, ["internal/a", "internal/b"], "cmd.go")),
                ["internal/a/cmd.go", "internal/b/cmd.go"],
            )
            self.assertEqual(
                sorted(resolve(root, ["internal/b", "internal/a"], "cmd.go")),
                ["internal/a/cmd.go", "internal/b/cmd.go"],
            )

    def test_overlapping_bases_same_file_is_unique(self):
        # A file reachable from two overlapping bases is one file, not ambiguous.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/a/sub/x.go", 5)
            self.assertEqual(
                resolve(root, ["internal/a", "internal/a/sub"], "x.go"),
                ["internal/a/sub/x.go"],
            )

    def test_ambiguous_within_one_base(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/a/sub1/pool.go", 5)
            write(root, "internal/a/sub2/pool.go", 5)
            self.assertEqual(len(resolve(root, ["internal/a"], "pool.go")), 2)


class TestCheckDigest(unittest.TestCase):
    def test_good_anchor_in_range(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/x/peer.go", 50)
            p = digest(
                root,
                "x.md",
                "<!-- digest-base: internal/x -->\nsee `peer.go:20` and `peer.go:10-40`\n",
            )
            errors, resolved = check_digest(root, p)
            self.assertEqual(errors, [])
            self.assertEqual(len(resolved), 2)

    def test_line_out_of_range(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/x/peer.go", 10)
            p = digest(root, "x.md", "<!-- digest-base: internal/x -->\n`peer.go:99`\n")
            errors, _ = check_digest(root, p)
            self.assertEqual(len(errors), 1)
            self.assertIn("out of range", errors[0]["problem"])

    def test_missing_file_with_line(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "internal" / "x").mkdir(parents=True)
            p = digest(root, "x.md", "<!-- digest-base: internal/x -->\n`gone.go:3`\n")
            errors, _ = check_digest(root, p)
            self.assertEqual(len(errors), 1)
            self.assertIn("not found", errors[0]["problem"])

    def test_ambiguous_with_line_errors(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/x/a/wire.go", 20)
            write(root, "internal/x/b/wire.go", 20)
            p = digest(root, "x.md", "<!-- digest-base: internal/x -->\n`wire.go:5`\n")
            errors, _ = check_digest(root, p)
            self.assertEqual(len(errors), 1)
            self.assertIn("ambiguous", errors[0]["problem"])

    def test_broken_full_path_link_errors(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            p = digest(root, "x.md", "see also `docs/architecture/gone.md`\n")  # <!-- doc-links: ignore (fixture path, deliberately absent) -->
            errors, _ = check_digest(root, p)
            self.assertEqual(len(errors), 1)
            self.assertIn("does not exist", errors[0]["problem"])

    def test_bare_noline_mention_skipped(self):
        # `register.go` bare and ambiguous, no line -> informal, not an error.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/x/a/register.go", 5)
            write(root, "internal/x/b/register.go", 5)
            p = digest(
                root,
                "x.md",
                "<!-- digest-base: internal/x -->\neach plugin has a `register.go`\n",
            )
            errors, _ = check_digest(root, p)
            self.assertEqual(errors, [])

    def test_reversed_range_errors(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/x/peer.go", 50)
            p = digest(
                root, "x.md", "<!-- digest-base: internal/x -->\n`peer.go:40-10`\n"
            )
            errors, _ = check_digest(root, p)
            self.assertEqual(len(errors), 1)
            self.assertIn("reversed", errors[0]["problem"])

    def test_missing_base_header_with_lined_anchor(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "internal/x/peer.go", 50)
            p = digest(root, "x.md", "no header here\n`peer.go:20`\n")
            errors, _ = check_digest(root, p)
            self.assertEqual(len(errors), 1)
            self.assertIn("digest-base", errors[0]["problem"])


if __name__ == "__main__":
    unittest.main()
