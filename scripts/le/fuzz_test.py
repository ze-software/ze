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

import io
import re
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.application.fuzz import (
    FUZZTIME,
    SKIP_DIRS,
    TIMEOUT,
    Options,
    Target,
    action,
    discover,
)
from le.paths import REPO_ROOT

# Go's own rule for a fuzz target name, restated rather than imported: `Fuzz`
# alone, or `Fuzz` followed by a character that is not a lower-case letter.
# `\w+` was wrong in both directions -- it took `func Fuzzing(`, which is not a
# target, and missed the bare `func Fuzz(`, which is one. A second opinion that
# uses a DIFFERENT definition of the thing is not independence, it is a second
# bug waiting for the first name that separates them.
_FUZZ_FUNC = re.compile(r'^func (Fuzz(?:[A-Z][A-Za-z0-9_]*)?)\s*\(', re.MULTILINE)


def _grep_fuzz_names(root: Path) -> set[str]:
    """Every top-level `func Fuzz*` under internal/, found independently.

    Deliberately NOT a call into `discover`: this is the second opinion the
    equality above compares against, so it repeats the walk rather than
    sharing it. It repeats the SKIP set too, from `fuzz.SKIP_DIRS`, because
    excluding a different set of directories would red the equality on correct
    code the moment a `tmp/` or `node_modules/` appeared under internal/.
    """
    names: set[str] = set()
    for path in (root / 'internal').rglob('*_test.go'):
        if any(part in SKIP_DIRS for part in path.relative_to(root).parts):
            continue
        names.update(_FUZZ_FUNC.findall(path.read_text(encoding='utf-8', errors='replace')))
    return names


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

    def test_the_real_tree_matches_an_independent_grep(self) -> None:
        """Equality against ground truth, not a threshold.

        The deleted `fuzz_targets_test.py` compared discovery against its own
        `func Fuzz` grep of the real tree. A `> 50` threshold does not replace
        that: a walk that silently drops one package still passes it, which is
        the failure the equality exists to catch.
        """
        discovered = {target.name for target in discover(REPO_ROOT)}
        ground = _grep_fuzz_names(REPO_ROOT)
        assert discovered == ground, (
            f'discovery and grep disagree: '
            f'only discovered {sorted(discovered - ground)}, '
            f'only grepped {sorted(ground - discovered)}'
        )
        # Non-vacuity: an empty set would satisfy the equality above if the
        # grep broke in the same direction.
        assert len(discovered) > 50


class TestRunOnePassesTheCallerThrough(unittest.TestCase):
    """`ze-fuzz-test-one` hands FUZZ and PKG to Go untouched.

    An earlier version filtered discovery by exact equality on both, so the
    documented `PKG=./internal/component/bgp/wireu/...` exited 2 and a Go
    regexp in FUZZ stopped matching. Discovery enumerates ALL targets; it has
    no business narrowing one the caller already named.
    """

    def _argv(self, **kwargs: str) -> str:
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            code = action(Options(listing=True, timeout=TIMEOUT, **kwargs))
        assert code == 0
        return buffer.getvalue()

    def test_a_regexp_name_reaches_go_unaltered(self) -> None:
        printed = self._argv(name='FuzzParse.*')
        assert '-fuzz=FuzzParse.*' in printed, printed

    def test_a_wildcard_package_reaches_go_unaltered(self) -> None:
        printed = self._argv(package='./internal/component/bgp/wireu/...')
        assert './internal/component/bgp/wireu/...' in printed, printed

    def test_naming_one_does_not_consult_discovery(self) -> None:
        """A name no `func Fuzz` declares still runs: Go decides, not us."""
        printed = self._argv(name='FuzzNothingDeclaresThisName')
        assert '-fuzz=FuzzNothingDeclaresThisName' in printed, printed


class TestTheSweepStopsAtTheFirstFailure(unittest.TestCase):
    """The Make recipe gave each fuzzer its own line, so the first non-zero
    aborted the target and the rest never ran. Continuing would spend another
    ~13 minutes fuzzing after a crash is in hand, and bury the failure that
    matters under everything that followed it.
    """

    def _sweep(self, codes: list[int]) -> tuple[int, int]:
        """Run the sweep over len(codes) targets, each returning its code.

        Returns (exit code, how many targets actually ran).
        """
        targets = [Target(f'Fuzz{i}', f'./internal/p{i}') for i in range(len(codes))]
        calls: list[str] = []

        def fake_stream(argv: object, **_: object) -> int:
            calls.append(str(argv))
            return codes[len(calls) - 1]

        with (
            mock.patch('le.application.fuzz.discover', return_value=targets),
            mock.patch('le.application.fuzz.stream', side_effect=fake_stream),
            redirect_stdout(io.StringIO()),
        ):
            code = action(Options(fuzztime='1s', timeout=TIMEOUT))
        return code, len(calls)

    def test_a_failing_fuzzer_stops_the_sweep(self) -> None:
        code, ran = self._sweep([0, 3, 0, 0])
        assert code == 3, "the failing target's exit code must survive"
        assert ran == 2, f'the sweep must stop at the failure, ran {ran} of 4'

    def test_an_all_green_sweep_runs_every_target(self) -> None:
        """Non-vacuity: the stop above is a stop, not a sweep that never ran."""
        code, ran = self._sweep([0, 0, 0, 0])
        assert code == 0
        assert ran == 4


if __name__ == '__main__':
    unittest.main()
