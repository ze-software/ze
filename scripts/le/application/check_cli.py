"""CLI contract gates: YANG command against handler, ownership, grammar, config claims.

Ported from mk/check-cli.mk.

    ./le check-cli                        every check in the area
    ./le check-cli --list                 what each one is for
    ./le check-cli ze-cli-grammar-check   one of them
    ./le check-cli ze-cli-grammar-check --json

Every gate here runs through `go run` with the FULL feature tag set. That is
load-bearing rather than incidental: a reduced set compiles modules out, so the
gate would report on a smaller command surface than the one that ships and
report it as clean.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet
from le.devtools.toolchain import toolchain

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']

_go = toolchain().go_run

GATES = GateSet(
    area='check-cli',
    gates=(
        Gate(
            name='ze-command-contract-check',
            argv=tuple(_go('scripts/docvalid/commands.go')),
            json_flag='--json',
            why='every YANG command node has a handler, and every handler a node',
        ),
        Gate(
            name='ze-command-ownership-check',
            argv=tuple(_go('scripts/checks/command_ownership.go')),
            json_flag='--json',
            why='each command is owned by exactly one plugin or component',
        ),
        Gate(
            name='ze-cli-grammar-check',
            argv=tuple(_go('scripts/checks/cli_grammar.go')),
            json_flag='--json',
            why=(
                'every built-in command obeys the keyword-before-value grammar'
                ' (ai/rules/cli.md, R1-R8) and no .yang carries a --flag'
            ),
        ),
        Gate(
            name='ze-config-claims-check',
            argv=tuple(_go('scripts/checks/config_claims.go')),
            json_flag='--json',
            why=(
                'every config subtree an operator can write reaches a plugin config'
                ' root, a hub handler path, or a recorded exception, and every'
                ' declared root names a real schema node'
            ),
        ),
        Gate(
            name='ze-docs-pipe-operators-update',
            argv=tuple(_go('scripts/docvalid/doc_drift.go', '--write-generated')),
            why='regenerate the published pipe operator table from the operator catalog',
            writes=True,
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
