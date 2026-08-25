#!/usr/bin/env python3
"""Tests for the gate-area subprogram shape.

The case this file exists for is `test_the_standalone_route_reaches_the_module_action`.
`le/registry.py` claims the layout stops the two routes into a subprogram from
diverging, and for a while it did not: `gateapp.main` called `gateapp.action`
by name, so a module with its own action was reached by the dispatcher and
bypassed by `python3 -m le.application.<area>`.

`registry_test.py` could not catch it. `TestBothRoutesAgree` compares the
OPTIONS both routes build, and those agreed perfectly. What differed was the
function the options were then handed to, which no options comparison can see.
"""

from __future__ import annotations

import argparse
import io
import sys
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le import gateapp
from le.application import check_rules
from le.devtools.gate import Gate, GateSet

ALPHA = Gate(name='ze-alpha-check', argv=('true',), why='first')
BETA = Gate(name='ze-beta-check', argv=('true',), why='second')
WRITER = Gate(name='ze-gamma-update', argv=('true',), why='third', writes=True)
JSONNY = Gate(name='ze-delta-check', argv=('true',), why='fourth', json_flag='--json')

SET = GateSet(area='demo', gates=(ALPHA, BETA, WRITER, JSONNY))


def _options(argv: list[str]) -> gateapp.Options:
    parser = argparse.ArgumentParser()
    gateapp.add_arguments(parser, SET)
    return gateapp.options(parser.parse_args(argv))


class TestSelection(unittest.TestCase):
    def test_no_name_runs_every_check_and_no_writer(self) -> None:
        """A bare `le <area>` must not regenerate anything.

        The checks and the generators were separate targets in Make, and a
        reader who types the area name expects the reporting half.
        """
        with (
            mock.patch.object(gateapp, 'run_all', return_value=[]) as ran,
            redirect_stdout(io.StringIO()),
        ):
            gateapp.action(_options([]), SET)
        # Every non-writing gate, the one with a --json variant included: a
        # JSON rendering does not make a check something other than a check.
        assert [g.name for g in ran.call_args.args[0]] == [
            'ze-alpha-check',
            'ze-beta-check',
            'ze-delta-check',
        ]

    def test_write_runs_the_generators_only(self) -> None:
        with (
            mock.patch.object(gateapp, 'run_all', return_value=[]) as ran,
            redirect_stdout(io.StringIO()),
        ):
            gateapp.action(_options(['--write']), SET)
        assert [g.name for g in ran.call_args.args[0]] == ['ze-gamma-update']

    def test_a_named_gate_runs_alone_even_when_it_writes(self) -> None:
        with (
            mock.patch.object(gateapp, 'run_all', return_value=[]) as ran,
            redirect_stdout(io.StringIO()),
        ):
            gateapp.action(_options(['ze-gamma-update']), SET)
        assert [g.name for g in ran.call_args.args[0]] == ['ze-gamma-update']

    def test_an_unknown_name_is_refused_and_lists_the_real_ones(self) -> None:
        """A typo must not silently run nothing and exit 0."""
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            code = gateapp.action(_options(['ze-nope']), SET)
        assert code == 2
        assert 'ze-alpha-check' in buffer.getvalue()

    def test_a_failing_gate_fails_the_run_and_is_named(self) -> None:
        buffer = io.StringIO()
        with (
            mock.patch.object(gateapp, 'run_all', return_value=['ze-beta-check']),
            redirect_stdout(buffer),
        ):
            code = gateapp.action(_options([]), SET)
        assert code == 1
        assert 'ze-beta-check' in buffer.getvalue()


class TestJson(unittest.TestCase):
    def test_json_over_several_gates_is_refused(self) -> None:
        """Two JSON documents interleaved on one stream parse as neither."""
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            code = gateapp.action(_options(['--json']), SET)
        assert code == 2
        assert 'exactly one gate' in buffer.getvalue()

    def test_the_json_flag_is_appended_to_the_gates_argv(self) -> None:
        assert JSONNY.command(as_json=True) == ['true', '--json']

    def test_a_gate_with_no_json_variant_ignores_the_flag(self) -> None:
        """Better than inventing a flag the underlying tool would reject."""
        assert ALPHA.command(as_json=True) == ['true']


class TestBothRoutesReachTheSameAction(unittest.TestCase):
    """The property the whole layout is for, checked where it actually broke.

    `registry_test.TestBothRoutesAgree` compares the OPTIONS each route builds.
    Those agreed while the routes ran different functions, so it could never
    have caught this.
    """

    def test_the_standalone_route_reaches_the_module_action(self) -> None:
        seen: list[gateapp.Options] = []

        def spy(opts: gateapp.Options) -> int:
            seen.append(opts)
            return 0

        with redirect_stdout(io.StringIO()):
            assert gateapp.main(['--write'], SET, None, run=spy) == 0

        assert len(seen) == 1, 'the module action must be the one that runs'
        assert seen[0].write is True

    def test_without_run_the_shared_action_is_used(self) -> None:
        """The default keeps a pure-table module to one line."""
        with (
            mock.patch.object(gateapp, 'run_all', return_value=[]) as ran,
            redirect_stdout(io.StringIO()),
        ):
            gateapp.main([], SET, None)
        assert ran.called

    def test_check_rules_standalone_write_uses_its_own_order(self) -> None:
        """The live case. Declaration order puts index before condensed;
        WRITE_ORDER puts condensed first, because both parse what the render
        writes and the order they were declared in is not a decision anybody
        made.
        """
        with (
            mock.patch.object(check_rules, 'run_all', return_value=[]) as ran,
            redirect_stdout(io.StringIO()),
        ):
            check_rules.main(['--write'])
        ordered = [g.name for g in ran.call_args.args[0]]
        assert ordered == list(check_rules.WRITE_ORDER)
        assert ordered[0] == 'ze-rules-render-update', 'the render must run first'


class TestListing(unittest.TestCase):
    def test_list_prints_every_gate_and_its_reason(self) -> None:
        """`why` is data so that this is possible; Make had it only in comments."""
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            code = gateapp.action(_options(['--list']), SET)
        listed = buffer.getvalue()
        assert code == 0
        for gate in SET.gates:
            assert gate.name in listed
            assert gate.why in listed

    def test_list_runs_no_gate(self) -> None:
        with mock.patch.object(gateapp, 'run_all') as ran, redirect_stdout(io.StringIO()):
            gateapp.action(_options(['--list']), SET)
        ran.assert_not_called()

    def test_every_gate_states_a_reason(self) -> None:
        """A gate with no stated reason is the thing the port existed to fix."""
        for gate in check_rules.GATES.gates:
            assert gate.why.strip(), gate.name


if __name__ == '__main__':
    unittest.main()
