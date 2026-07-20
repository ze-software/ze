#!/usr/bin/env python3
"""Unit tests for perf-suggest.py, the local perf-run nudge.

Run under `go test` via scripts/dev/python_tests_test.go (TestPythonUnitTests),
so this file needs no separate wiring. Every case runs in a throwaway git repo
with the real hot-path prefixes recreated, so the detector's git and marker
logic is exercised for real, not mocked.
"""

from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = (Path(__file__).parent / "perf-suggest.py").read_text(encoding="utf-8")


def _run(cmd, cwd):
    return subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, check=False)


class PerfSuggestState(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.root = Path(self.dir.name)
        for sub in (
            "internal/component/bgp/reactor",
            "internal/component/bgp/plugins/rs",
            "internal/core/bgp",
            "scripts/dev",
            "tmp",
        ):
            (self.root / sub).mkdir(parents=True)
        (self.root / "scripts/dev/perf-suggest.py").write_text(SCRIPT, encoding="utf-8")
        self.hot = self.root / "internal/component/bgp/reactor/r.go"
        self.hot.write_text("package reactor\n", encoding="utf-8")
        (self.root / "other.go").write_text("package other\n", encoding="utf-8")
        for c in (
            ["git", "init", "-q", "."],
            ["git", "config", "user.email", "t@t"],
            ["git", "config", "user.name", "t"],
            ["git", "add", "-A"],
            ["git", "commit", "-qm", "init"],
        ):
            _run(c, self.root)

    def tearDown(self):
        self.dir.cleanup()

    def _commit(self, msg):
        _run(["git", "add", "-A"], self.root)
        _run(["git", "commit", "-qm", msg], self.root)

    def _suggests(self) -> bool:
        r = _run([sys.executable, "scripts/dev/perf-suggest.py"], self.root)
        self.assertEqual(r.returncode, 0, f"must always exit 0: {r.stderr}")
        return "perf-suggest:" in r.stderr

    def _record(self):
        r = _run([sys.executable, "scripts/dev/perf-suggest.py", "--record"], self.root)
        self.assertEqual(r.returncode, 0)

    def test_clean_tree_is_silent(self):
        self.assertFalse(self._suggests())

    def test_non_hot_edit_is_silent(self):
        (self.root / "other.go").write_text("package other // x\n", encoding="utf-8")
        self.assertFalse(self._suggests())

    def test_uncommitted_hot_edit_suggests(self):
        self.hot.write_text("package reactor // x\n", encoding="utf-8")
        self.assertTrue(self._suggests())

    def test_route_server_fast_path_is_covered(self):
        # Regression guard for review Finding 2: the sole ze perf config enables
        # rs-fast-path on every peer, so plugins/rs/server_forward.go is the
        # measured throughput path. It was missing from HOT_PATH_PREFIXES, so
        # the nudge stayed silent on exactly the regression it exists to catch.
        rs = self.root / "internal/component/bgp/plugins/rs/server_forward.go"
        rs.write_text("package rs // edited\n", encoding="utf-8")
        self.assertTrue(self._suggests())

    def test_committed_hot_no_baseline_no_upstream_is_silent(self):
        # No trusted point and no upstream: working-tree only. Quiet on a fresh
        # or untracked checkout rather than nagging about inherited code.
        self.hot.write_text("package reactor // x\n", encoding="utf-8")
        self._commit("hot")
        self.assertFalse(self._suggests())

    def test_committed_hot_with_upstream_merge_base_suggests(self):
        # The branch's committed-but-unperfed work IS flagged once an upstream
        # gives a merge-base to measure against -- the gap that made the nudge
        # "forget" a hot change the moment it was committed.
        self.hot.write_text("package reactor // x\n", encoding="utf-8")
        self._commit("hot")
        _run(["git", "branch", "base-point", "HEAD~1"], self.root)
        _run(["git", "remote", "add", "origin", "."], self.root)
        _run(["git", "update-ref", "refs/remotes/origin/main", "base-point"], self.root)
        _run(["git", "branch", "--set-upstream-to=origin/main"], self.root)
        self.assertTrue(self._suggests())

    def test_baseline_covers_committed_change(self):
        self.hot.write_text("package reactor // x\n", encoding="utf-8")
        self._commit("hot")
        self._record()
        self.assertFalse(self._suggests())

    def test_new_commit_past_baseline_suggests(self):
        self.hot.write_text("package reactor // x\n", encoding="utf-8")
        self._commit("hot")
        self._record()
        self.hot.write_text("package reactor // y\n", encoding="utf-8")
        self._commit("hot2")
        self.assertTrue(self._suggests())


if __name__ == "__main__":
    unittest.main()
