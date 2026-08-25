"""The subprogram shape a gate area has, written once.

Most `mk/*.mk` files were a list of named checks and nothing else. Their `le`
counterparts are the same list plus four one-line functions, and those four
would be identical in twenty modules. They live here instead, and each area
module calls them with its own `GateSet`:

    def add_arguments(parser): gateapp.add_arguments(parser, GATES)
    def options(namespace):    return gateapp.options(namespace)
    def action(opts):          return gateapp.action(opts, GATES)
    def main(argv=None):       return gateapp.main(argv, GATES, __doc__)

Explicit calls rather than generated attributes: a reader of an area module
sees the four names the registry requires, and `le/registry.py` still finds
them by `hasattr` with nothing dynamic in the way.

Running no gate name runs the whole set. That is what `make ze-<area>-check`
did when the area had an aggregate target, and it is the common case.
"""

from __future__ import annotations

import argparse
from collections.abc import Sequence
from dataclasses import dataclass

from le.console import echo
from le.devtools.gate import GateSet, run_all, run_gate

__all__ = ['Options', 'action', 'add_arguments', 'main', 'options']


@dataclass(frozen=True)
class Options:
    """Everything a gate area's action needs.

    `names` empty means every gate in the set. `write` selects the generators
    rather than the checks, which is the `-update` half of the old target pairs.
    """

    names: tuple[str, ...] = ()
    as_json: bool = False
    write: bool = False
    listing: bool = False


def add_arguments(parser: argparse.ArgumentParser, gates: GateSet) -> None:
    """Declare the flags every gate area accepts."""
    parser.add_argument(
        'names',
        nargs='*',
        metavar='<gate>',
        help='gates to run; with none, every check in the area runs',
    )
    parser.add_argument(
        '--list',
        dest='listing',
        action='store_true',
        help='print every gate in this area and what it is for',
    )
    parser.add_argument(
        '--json',
        dest='as_json',
        action='store_true',
        help='ask each gate for its machine-readable report, where it has one',
    )
    parser.add_argument(
        '--write',
        action='store_true',
        help='run the generators of this area instead of its checks',
    )
    parser.set_defaults(gateset=gates)


def options(namespace: argparse.Namespace) -> Options:
    """Turn the parsed namespace into the typed value `action` takes."""
    return Options(
        names=tuple(namespace.names),
        as_json=bool(namespace.as_json),
        write=bool(namespace.write),
        listing=bool(namespace.listing),
    )


def action(opts: Options, gates: GateSet) -> int:
    """Run what the options select. Returns the process exit code."""
    if opts.listing:
        echo(f'{gates.area}:')
        gates.render_list()
        return 0

    if opts.names:
        selected = []
        for name in opts.names:
            gate = gates.find(name)
            if gate is None:
                echo(f'no such gate in {gates.area}: {name}')
                echo(f'try one of: {", ".join(gates.names())}')
                return 2
            selected.append(gate)
        chosen = tuple(selected)
    else:
        chosen = gates.writers() if opts.write else gates.checks()

    if not chosen:
        kind = 'generator' if opts.write else 'check'
        echo(f'{gates.area} declares no {kind}')
        return 0

    if opts.as_json:
        # One gate at a time: two JSON documents interleaved on one stream is
        # not parseable by the caller that asked for JSON.
        if len(chosen) != 1:
            echo('--json takes exactly one gate name')
            return 2
        return run_gate(chosen[0], as_json=True)

    failed = run_all(chosen)
    echo()
    if failed:
        echo(f'Failed: {", ".join(failed)}')
        return 1
    echo(f'{gates.area}: {len(chosen)} gate(s) passed.')
    return 0


def main(argv: Sequence[str] | None, gates: GateSet, doc: str | None = None) -> int:
    """Standalone entry for a gate area. Parses, then calls `action`."""
    parser = argparse.ArgumentParser(prog=f'le {gates.area}', description=doc)
    add_arguments(parser, gates)
    return action(options(parser.parse_args(argv)), gates)
