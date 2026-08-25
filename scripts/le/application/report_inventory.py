"""Repository reports: inventory, the command list, the rules readouts, the journal.

Ported from mk/report-inventory.mk.

    ./le report-inventory                          every report
    ./le report-inventory --list                   what each one is for
    ./le report-inventory ze-inventory             one of them
    ./le report-inventory ze-inventory --json

A report is a check here rather than a third kind: some of them exit non-zero
(ze-rules-gate-map-report fails on a dangling point id, ze-journal-report on a
problem class with two rows), and the ones that only print exit 0, which is what
`checks()` already means.

`ze-spec-status` and `ze-spec-status-json` did NOT move. The first is a
sequence of two programs and two headings where the second program's failure is
deliberately swallowed (`|| true`), which a Gate cannot express, and the second
is its JSON twin: splitting the pair would leave a lone `-json` gate whose
non-JSON half is a Make recipe.

ZE_CONTEXT_CAP and ZE_SESSION moved with ze-token-economy-report, from
`$(if ...)` to the environment. A variable set on the make command line reaches
the recipe's environment, so `make ze-token-economy-report ZE_CONTEXT_CAP=150000`
and `ZE_CONTEXT_CAP=150000 ./le report-inventory ze-token-economy-report` build
the same argv.
"""

from __future__ import annotations

import argparse
import os
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet
from le.devtools.toolchain import toolchain

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']

_go = toolchain().go_run

# The Makefile's `ZE_CONTEXT_CAP ?= 200000`, which is also token_economy.py's
# own DEFAULT_CAP. Named once here now that the Make variable is gone.
DEFAULT_CONTEXT_CAP = '200000'


def _py(script: str, *args: str) -> tuple[str, ...]:
    return ('python3', f'scripts/dev/{script}', *args)


def _token_economy_argv() -> tuple[str, ...]:
    """`token_economy.py` with the cap and the session filter the caller set."""
    cap = os.environ.get('ZE_CONTEXT_CAP', '').strip() or DEFAULT_CONTEXT_CAP
    session = os.environ.get('ZE_SESSION', '').strip()
    args = ['--cap', cap]
    if session:
        args.extend(('--session', session))
    return _py('token_economy.py', *args)


GATES = GateSet(
    area='report-inventory',
    gates=(
        Gate(
            name='ze-inventory',
            argv=tuple(_go('scripts/inventory/inventory.go')),
            json_flag='--json',
            why='registry-backed plugin, command, YANG, test and package inventory',
        ),
        Gate(
            name='ze-command-list',
            argv=tuple(_go('scripts/inventory/commands.go')),
            json_flag='--json',
            why='every registered command, by verb, read from the live handlers and schemas',
        ),
        Gate(
            name='ze-rules-gate-map-report',
            argv=_py('rules_points.py', 'coverage'),
            why=(
                'which rule point each hook check enforces. Gated and ungated are'
                ' MEASUREMENTS and exit 0: an ungated point is a rule no machine enforces'
                ' yet. Dangling FAILS, and so do a check that named a point at HEAD and'
                ' declares none now, a rule holding fewer points than HEAD with no row in'
                ' ai/rules/points/RETIRED.md, and a rationale or excepted-by naming nothing'
            ),
        ),
        Gate(
            name='ze-rules-payload-report',
            argv=_py('rules_condensed.py', '--payload'),
            why=(
                'what a session actually loads -- ai/INSTRUCTIONS.md, TRIGGERS.md and'
                ' CORE.md -- measured against the token budget and the digest it replaces'
            ),
        ),
        Gate(
            name='ze-rules-router-report',
            argv=_py('rules_router.py'),
            json_flag='--json',
            why=(
                'over every open spec Task section in plan/, which rules the trigger index'
                ' would surface and which BLOCKING rules no task surfaces at all. The second'
                ' set is what the always-on core exists to protect, and the generator derives'
                ' the core from it'
            ),
        ),
        Gate(
            name='ze-token-economy-report',
            argv=_token_economy_argv(),
            why=(
                'where this repository session spends its tokens: API calls, the context'
                ' carried at each one, the size histogram and a capped-context'
                ' counterfactual. Reads the machine-local transcript store, so a checkout'
                ' with none reports that and exits 0. Token counts only, never money'
            ),
        ),
        Gate(
            name='ze-journal-report',
            argv=_py('journal.py'),
            why=(
                'every problem class in plan/journal/ with 2+ occurrences, its row count and'
                ' the span between first and last date. Prints nothing when every class has'
                ' one row'
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
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
