"""Chaos-testing gates: the simulator's own packages, its CLI surface, its linter.

Ported from mk/test-chaos.mk. Three targets moved; that file's header lists
what stayed and why.

    ./le test-chaos                        every check
    ./le test-chaos --list                 what each one is for
    ./le test-chaos ze-chaos-lint          one of them

ze-chaos-unit-test is TWO runs, and the second is the point of this module.
The orchestrator's CLI sits in cmd/ze behind //go:build ze_chaos, and the
normal tag set carries no ze_chaos, so `./internal/chaos/...` reaches none of
it: `go test` compiles cmd/ze with ze_chaos_main_test.go excluded and says
nothing about it. Eleven tests sat in that state and the file had stopped
COMPILING -- chaosRun called orchestrator.CLIRun, which is unexported now
(plan/journal/gate-excludes-part-of-its-population.md). It goes through
registry.LookupRoot("chaos") instead, a runtime dependency on the registration
ze_chaos_run.go performs, and ze-chaos-cli-unit-test is what judges it.

ze_bgp is in that tag set because the orchestrator drives an in-process BGP
reactor. ze-chaos-build forces the same tag for the same reason.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet
from le.devtools.toolchain import toolchain

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']

# Everything the chaos orchestrator is, minus the CLI surface below.
CHAOS_PACKAGES = './internal/chaos/...'

# The tag set that reaches the orchestrator's CLI. Not the feature set: this is
# the ze_chaos build, and ze_bgp is what its in-process reactor needs.
CHAOS_CLI_TAGS = 'ze_core ze_bgp ze_chaos'


def _cli_test() -> tuple[str, ...]:
    """`go test -timeout $(GO_TEST_TIMEOUT) -tags '$(CHAOS_CLI_TAGS)' ./cmd/ze`.

    Built by hand rather than through `go_test`, which carries the feature tag
    set. A feature build of cmd/ze compiles the chaos CLI out, which is the
    whole thing this run exists to reach.
    """
    return ('go', 'test', '-timeout', toolchain().timeout, '-tags', CHAOS_CLI_TAGS, './cmd/ze')


GATES = GateSet(
    area='test-chaos',
    gates=(
        Gate(
            name='ze-chaos-lint',
            argv=('golangci-lint', 'run', '-j', str(toolchain().procs), CHAOS_PACKAGES),
            why='the chaos orchestrator lints clean, under the same two ceilings every run has',
        ),
        Gate(
            name='ze-chaos-unit-test',
            argv=tuple(toolchain().go_test(CHAOS_PACKAGES, race=True)),
            why='the chaos simulator: fault injection, scheduling, the in-process reactor',
        ),
        Gate(
            name='ze-chaos-cli-unit-test',
            argv=_cli_test(),
            why=(
                "the orchestrator's CLI surface, which only a ze_chaos build compiles;"
                ' the default tag set excludes it and reports nothing'
            ),
        ),
    ),
)


def _environment(gate: Gate) -> dict[str, str]:
    """The environment one gate runs under, read off the command it runs.

    The linter takes the soft heap ceiling and no GOMAXPROCS, because its share
    of the box is the `-j` in its own argv. A race run takes cgo, which it
    cannot link without. Everything else inherits the repository's
    CGO_ENABLED=0.
    """
    if gate.argv[0] == 'golangci-lint':
        return toolchain().environment(memlimit=True)
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
