"""Component-group unit suites, race-instrumented.

Ported from mk/test-unit.mk. Only the five COMPONENT-GROUP targets moved; that
file's header lists what stayed behind and why.

    ./le test-unit                     every group
    ./le test-unit --list              what each group covers
    ./le test-unit ze-unit-core-test   one of them

Each group covers one logical area, and the five together are what
`ze-unit-test` runs over the whole package list. They exist so a session
developing one area tests that area rather than the tree.

A bare `go test ./internal/...` typed into a shell is NOT this command, which
is why the group targets are named at all. Four things come from here and none
of them come from a shell run (ai/rules/commands.md):

  The tag set is `ze_core` plus every gate in feature-gates.txt, so the suite
  compiles the feature surface that ships. A reduced set compiles modules out
  and the suite then reports on a smaller product than the one that ships.

  GOCACHE is the checkout's own cache rather than the user's, so the run starts
  warm, shares its entries with `ze-precommit-verify`, and leaves that cache
  warmer than it found it.

  The timeout is 20 minutes per test binary. It is a hang catcher rather than a
  wait: no test is expected to approach it, and one that does is a defect to
  fix rather than a number to raise.

  GOMAXPROCS is a quarter of the cores. `go test -p` defaults to it, so it also
  caps how many packages compile at once, which is where the memory pressure
  is. Several sessions share this checkout and this machine.

CGO_ENABLED is 1 for exactly these runs and 0 everywhere else in the tree. The
race detector cannot build without it, and a race test binary never ships or
serves as release evidence, so these are the sole cgo compilation path.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet
from le.devtools.toolchain import toolchain

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']


def _race(pattern: str) -> tuple[str, ...]:
    """`$(GO_TEST_RACE) <pattern>`, argument for argument."""
    return tuple(toolchain().go_test(pattern, race=True))


GATES = GateSet(
    area='test-unit',
    gates=(
        Gate(
            name='ze-unit-bgp-test',
            argv=_race('./internal/component/bgp/...'),
            why='the BGP component group: reactor, fsm, wire, message, attribute (~1:30)',
        ),
        Gate(
            name='ze-unit-core-test',
            argv=_race('./internal/core/...'),
            why='the core leaf libraries every tier above depends on (~30s)',
        ),
        Gate(
            name='ze-unit-plugins-test',
            argv=_race('./internal/plugins/...'),
            why='the system plugins: DHCP, NTP, static, firewall, the CLI verb providers (~40s)',
        ),
        Gate(
            name='ze-unit-config-test',
            argv=_race('./internal/component/config/...'),
            why='the YANG-modeled config pipeline: file, tree, resolve (~20s)',
        ),
        Gate(
            name='ze-unit-cli-test',
            argv=_race('./internal/component/cli/...'),
            why='the CLI: modes, completion, diff, commit, dashboard (~10s)',
        ),
    ),
)


def _environment(gate: Gate) -> dict[str, str]:
    """The environment one gate runs under, read off the command it runs.

    Derived rather than declared beside the gate, so a gate that stops being a
    race run stops asking for cgo with no second edit.
    """
    return toolchain().environment(cgo='-race' in gate.argv, procs=True)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select, each gate under its own Go environment.

    `_environment` is what this area adds: the shared helper's default
    carries the toolchain pins, and these gates need more than that
    (CGO_ENABLED for a race build, GOMAXPROCS for a test run).
    """
    return gateapp.action(opts, GATES, env=_environment)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
