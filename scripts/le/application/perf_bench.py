"""Performance benchmark reports.

Ported from mk/perf-bench.mk, of which one target could move.

    ./le perf-bench                            every check in the area
    ./le perf-bench --list                     what each one is for
    ./le perf-bench ze-perf-suggestion-report  one of them

The other four are still recipes in mk/perf-bench.mk, each for a reason a Gate
cannot express:

    ze-perf-build           an alias for the $(ZEBIN_PERF) file target, whose one
                            build recipe lives in the root Makefile
    ze-perf-bench           passes through the shared-machine admission wrapper
                            (scripts/dev/ze-run.sh), which re-enters make for the
                            `_ze-perf-bench-impl` half, and needs Docker
    ze-perf-report          a shell glob (test/perf/results/*.json) and a
                            prerequisite on the built binary
    ze-perf-history-record  a `for` loop over that glob
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']

GATES = GateSet(
    area='perf-bench',
    gates=(
        Gate(
            name='ze-perf-suggestion-report',
            argv=('python3', 'scripts/dev/perf-suggest.py'),
            why=(
                'suggest a perf run when BGP data-plane code changed since the last one.'
                ' A NUDGE, never a gate -- always exits 0. The heavy suite needs Docker and'
                ' minutes, so it is not run every edit; this notices when a Docker perf run'
                ' is overdue on THIS machine, beside the nightly Docker-free regression check'
            ),
        ),
    ),
)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    return gateapp.action(opts, GATES)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
