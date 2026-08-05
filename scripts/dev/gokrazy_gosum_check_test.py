#!/usr/bin/env python3
"""Tests for gokrazy_gosum_check."""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import gokrazy_gosum_check as gg  # noqa: E402


class TestBuilddirGosums(unittest.TestCase):
    """Selecting the files to check."""

    def test_selects_only_tracked_builddir_gosums(self):
        files = [
            "go.sum",
            "gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.sum",
            "gokrazy/ze/builddir/github.com/rtr7/kernel/go.sum",
            "gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod",
            "gokrazy/modcache/github.com/gokrazy/gokrazy@v0/go.sum",
            "internal/foo/go.sum",
        ]
        self.assertEqual(
            gg.builddir_gosums(files),
            [
                "gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.sum",
                "gokrazy/ze/builddir/github.com/rtr7/kernel/go.sum",
            ],
        )

    def test_excludes_the_root_gosum(self):
        """The root is the thing compared against, never a thing compared."""
        self.assertEqual(gg.builddir_gosums(["go.sum"]), [])

    def test_excludes_the_modcache(self):
        """gokrazy/modcache is a vendored cache, not a builddir module."""
        self.assertEqual(
            gg.builddir_gosums(["gokrazy/modcache/github.com/x@v1/go.sum"]), []
        )


class TestParseGosum(unittest.TestCase):
    """Reading a go.sum."""

    def _write(self, text):
        handle = tempfile.NamedTemporaryFile(
            "w", suffix=".sum", delete=False, encoding="utf-8"
        )
        handle.write(text)
        handle.close()
        self.addCleanup(os.unlink, handle.name)
        return handle.name

    def test_keeps_zip_and_gomod_lines_apart(self):
        """`v1.2.3` and `v1.2.3/go.mod` are different keys with different hashes.

        Collapsing them would compare a zip hash against a go.mod hash and
        report a conflict on every module in the file.
        """
        path = self._write(
            "example.com/m v1.2.3 h1:AAA=\nexample.com/m v1.2.3/go.mod h1:BBB=\n"
        )
        entries = gg.parse_gosum(path)
        self.assertEqual(entries[("example.com/m", "v1.2.3")], "h1:AAA=")
        self.assertEqual(entries[("example.com/m", "v1.2.3/go.mod")], "h1:BBB=")

    def test_ignores_malformed_lines(self):
        path = self._write("\nexample.com/m v1.0.0 h1:AAA=\ngarbage\n")
        self.assertEqual(len(gg.parse_gosum(path)), 1)


class TestFindConflicts(unittest.TestCase):
    """The one condition that cannot be legitimate."""

    def setUp(self):
        self.root = {
            ("example.com/shared", "v1.0.0"): "h1:SHARED=",
            ("example.com/skewed", "v1.0.0"): "h1:OLD=",
        }

    def _reader(self, mapping):
        return lambda path: mapping[path]

    def test_matching_hash_is_not_a_conflict(self):
        read = self._reader(
            {"b/go.sum": {("example.com/shared", "v1.0.0"): "h1:SHARED="}}
        )
        self.assertEqual(gg.find_conflicts(self.root, ["b/go.sum"], read), [])

    def test_differing_hash_for_the_same_version_is_a_conflict(self):
        read = self._reader(
            {"b/go.sum": {("example.com/shared", "v1.0.0"): "h1:OTHER="}}
        )
        conflicts = gg.find_conflicts(self.root, ["b/go.sum"], read)
        self.assertEqual(len(conflicts), 1)
        path, module, version, root_digest, builddir_digest = conflicts[0]
        self.assertEqual(
            (path, module, version), ("b/go.sum", "example.com/shared", "v1.0.0")
        )
        self.assertEqual((root_digest, builddir_digest), ("h1:SHARED=", "h1:OTHER="))

    def test_module_absent_from_root_is_not_a_conflict(self):
        """Builddir modules legitimately depend on things the root does not.

        The packed programs are third-party (dhcp, ntp, randomd, serial-busybox,
        the rtr7 kernel), so flagging their private dependencies would make the
        gate fire on correct state.
        """
        read = self._reader({"b/go.sum": {("example.com/private", "v9.9.9"): "h1:X="}})
        self.assertEqual(gg.find_conflicts(self.root, ["b/go.sum"], read), [])

    def test_version_skew_is_not_a_conflict(self):
        """A different VERSION of the same module is normal, not a disagreement.

        Only the same (module, version) hashing two ways is unexplainable.
        """
        read = self._reader({"b/go.sum": {("example.com/skewed", "v2.0.0"): "h1:NEW="}})
        self.assertEqual(gg.find_conflicts(self.root, ["b/go.sum"], read), [])

    def test_reports_every_conflicting_file(self):
        read = self._reader(
            {
                "a/go.sum": {("example.com/shared", "v1.0.0"): "h1:A="},
                "b/go.sum": {("example.com/shared", "v1.0.0"): "h1:B="},
            }
        )
        conflicts = gg.find_conflicts(self.root, ["a/go.sum", "b/go.sum"], read)
        self.assertEqual(len(conflicts), 2)
        self.assertEqual({c[0] for c in conflicts}, {"a/go.sum", "b/go.sum"})


class TestRealTree(unittest.TestCase):
    """The check against the repository as it stands."""

    def test_the_tree_is_clean(self):
        """Measured 2026-08-05: 7 builddir files, 93 shared entries, 0 conflicts.

        A gate that has never been green on the real tree is a gate nobody can
        trust, so this pins that the tree passes today rather than only that the
        logic is sound on fixtures.
        """
        files = gg.tracked_files(
            os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
        )
        builddir = gg.builddir_gosums(files)
        self.assertTrue(builddir, "the tracked builddir go.sum files should exist")

        repo = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
        root = gg.parse_gosum(os.path.join(repo, gg.ROOT_GOSUM))
        conflicts = gg.find_conflicts(root, [os.path.join(repo, p) for p in builddir])
        self.assertEqual(conflicts, [], "the tracked tree must have no hash conflict")


if __name__ == "__main__":
    unittest.main()
