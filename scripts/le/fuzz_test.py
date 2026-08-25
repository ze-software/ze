#!/usr/bin/env python3
"""Tests for run-time fuzz-target discovery.

These replace `scripts/dev/fuzz_targets_test.py`, which tested the GENERATOR
that froze the target list into `mk/test-fuzz-targets.mk`. That file and its
generator are gone: `le fuzz` walks the tree when the fuzzers run, so there is
no fragment and no freshness to assert.

What survived the move is everything that was ever about BEHAVIOUR rather than
about the file. Both constraints below are Go fuzz requirements, and each was
learned from a real failure:

  An unanchored `-fuzz=Name` matches every target whose name it prefixes, and
  Go refuses with "matches more than one fuzz target".

  A `./...` package path matches every package under it, and Go refuses with
  "matches more than one package".

`test/weakened.md` claimed both properties survived as properties of
`Target.command`. Nothing asserted that until this file existed, so the claim
was true and unguarded. A reviewer caught it.
"""

from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.application.fuzz import FUZZTIME, TIMEOUT, Target, discover
from le.paths import REPO_ROOT


def _tree(**files: str) -> Path:
    """A throwaway checkout holding `internal/<path> = <content>`."""
    root = Path(tempfile.mkdtemp())
    for rel, text in files.items():
        path = root / 'internal' / rel.replace('__', '/')
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
    return root


class TestCommandShape(unittest.TestCase):
    """The two Go fuzz requirements, asserted on the command that runs."""

    def test_the_name_is_anchored(self) -> None:
        """`FuzzParseVPN` unanchored also matches `FuzzParseVPNAddPath`."""
        argv = Target(name='FuzzParseVPN', package='./internal/x').command()
        assert '-fuzz=^FuzzParseVPN$' in argv

    def test_the_package_is_an_exact_directory(self) -> None:
        """A tree with sibling packages (isis/{packet,yang}) refuses a wildcard."""
        argv = Target(name='FuzzX', package='./internal/plugins/isis/packet').command()
        assert './internal/plugins/isis/packet' in argv
        assert not any(part.endswith('/...') for part in argv)

    def test_no_discovered_package_is_a_wildcard(self) -> None:
        """The property over the REAL tree, not only over a constructed pair."""
        for target in discover():
            assert not target.package.endswith('...'), target.package

    def test_every_discovered_name_is_anchored_in_its_command(self) -> None:
        for target in discover():
            assert f'-fuzz=^{target.name}$' in target.command(), target.name

    def test_the_budget_is_carried(self) -> None:
        argv = Target(name='FuzzX', package='./internal/x').command()
        assert f'-fuzztime={FUZZTIME}' in argv
        assert f'-timeout={TIMEOUT}' in argv

    def test_a_longer_run_overrides_only_the_fuzztime(self) -> None:
        argv = Target(name='FuzzX', package='./internal/x').command(fuzztime='30s')
        assert '-fuzztime=30s' in argv
        assert f'-timeout={TIMEOUT}' in argv


class TestDiscovery(unittest.TestCase):
    def test_a_func_fuzz_is_found_with_its_exact_package(self) -> None:
        root = _tree(**{'a__b__x_test.go': 'package b\n\nfunc FuzzThing(f *testing.F) {}\n'})
        assert discover(root) == [Target(name='FuzzThing', package='./internal/a/b')]

    def test_several_targets_in_one_file_are_all_found(self) -> None:
        root = _tree(
            **{
                'a__x_test.go': 'package a\n\nfunc FuzzOne(f *testing.F) {}\nfunc FuzzTwo(f *testing.F) {}\n'
            }
        )
        assert [t.name for t in discover(root)] == ['FuzzOne', 'FuzzTwo']

    def test_a_file_with_no_fuzz_contributes_nothing(self) -> None:
        root = _tree(**{'a__x_test.go': 'package a\n\nfunc TestThing(t *testing.T) {}\n'})
        assert discover(root) == []

    def test_a_name_merely_beginning_with_fuzz_is_not_a_target(self) -> None:
        """Go's rule: `func Fuzz` or `func FuzzXxx` with Xxx upper-case.

        `func Fuzzy` is an ordinary function. The deleted generator's pattern
        was `Fuzz[A-Za-z0-9_]+`, which took it, so the committed fragment
        would have emitted `-fuzz=^Fuzzy$` against a package holding no such
        target and Go would have refused the run. No such function exists in
        the tree, so this was latent; the port corrected it rather than
        carrying it forward.
        """
        root = _tree(**{'a__x_test.go': 'package a\n\nfunc Fuzzy() {}\n'})
        assert discover(root) == []

    def test_a_bare_func_fuzz_is_a_target(self) -> None:
        """`func Fuzz(f *testing.F)` with no suffix is valid Go."""
        root = _tree(**{'a__x_test.go': 'package a\n\nfunc Fuzz(f *testing.F) {}\n'})
        assert [t.name for t in discover(root)] == ['Fuzz']

    def test_a_real_fuzz_signature_is_taken(self) -> None:
        root = _tree(**{'a__x_test.go': 'package a\n\nfunc FuzzReal(f *testing.F) {}\n'})
        assert [t.name for t in discover(root)] == ['FuzzReal']

    def test_an_indented_func_is_not_a_top_level_target(self) -> None:
        root = _tree(**{'a__x_test.go': 'package a\n\nfunc T() {\n\tfunc FuzzInner() {}\n}\n'})
        assert discover(root) == []

    def test_vendor_and_testdata_are_not_walked(self) -> None:
        """A vendored fuzz target is not ours to run."""
        root = _tree(
            **{
                'vendor__dep__x_test.go': 'package dep\n\nfunc FuzzVendored(f *testing.F) {}\n',
                'a__testdata__x_test.go': 'package a\n\nfunc FuzzFixture(f *testing.F) {}\n',
                'a__x_test.go': 'package a\n\nfunc FuzzReal(f *testing.F) {}\n',
            }
        )
        assert [t.name for t in discover(root)] == ['FuzzReal']

    def test_a_non_test_file_is_not_read(self) -> None:
        """Go fuzz targets live in _test.go files; anything else is not one."""
        root = _tree(**{'a__x.go': 'package a\n\nfunc FuzzNotATest(f *testing.F) {}\n'})
        assert discover(root) == []

    def test_the_order_is_package_then_name(self) -> None:
        """Stable, so an interrupted run is readable and a diff is reviewable."""
        root = _tree(
            **{
                'z__x_test.go': 'package z\n\nfunc FuzzB(f *testing.F) {}\n',
                'a__x_test.go': 'package a\n\nfunc FuzzZ(f *testing.F) {}\nfunc FuzzA(f *testing.F) {}\n',
            }
        )
        assert [(t.package, t.name) for t in discover(root)] == [
            ('./internal/a', 'FuzzA'),
            ('./internal/a', 'FuzzZ'),
            ('./internal/z', 'FuzzB'),
        ]

    def test_a_tree_with_no_internal_yields_nothing(self) -> None:
        assert discover(Path(tempfile.mkdtemp())) == []

    def test_the_real_tree_declares_targets(self) -> None:
        """A guard against the whole thing silently finding nothing.

        Every assertion above passes vacuously on an empty result, so a walk
        that stopped working would leave them all green.
        """
        assert len(discover(REPO_ROOT)) > 50


if __name__ == '__main__':
    unittest.main()
