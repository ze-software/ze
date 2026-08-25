"""The subprogram shape a gate area has, written once.

Most `mk/*.mk` files were a list of named checks and nothing else. Their `le`
counterparts are the same list plus four one-line functions, and those four
would be identical in twenty modules. They live here instead, and each area
module calls them with its own `GateSet`:

    def add_arguments(parser): gateapp.add_arguments(parser, GATES)
    def options(namespace):    return gateapp.options(namespace)
    def action(opts):          return gateapp.action(opts, GATES)
    def main(argv=None):       return gateapp.main(argv, GATES, __doc__, run=action)

A module whose `action` is the one-liner above may omit `run=action`. A module
that writes its OWN action MUST pass it, or the standalone route silently runs
the shared one instead. `run=action` is correct in both cases, so write it
always and the question never arises.

Explicit calls rather than generated attributes: a reader of an area module
sees the four names the registry requires, and `le/registry.py` still finds
them by `hasattr` with nothing dynamic in the way.

Running no gate name runs the whole set. That is what `make ze-<area>-check`
did when the area had an aggregate target, and it is the common case.
"""

from __future__ import annotations

import argparse
from collections.abc import Callable, Sequence
from dataclasses import dataclass

from le.console import echo
from le.devtools.gate import Gate, GateSet, run_gate
from le.devtools.toolchain import toolchain

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


def default_environment(gate: Gate) -> dict[str, str]:
    """The environment every gate runs under unless its area says otherwise.

    The Makefile exported GOCACHE, GOLANGCI_LINT_CACHE, CGO_ENABLED and
    GOTOOLCHAIN at the top of every run, so each recipe inherited them. `le`
    has no equivalent ambient step, and a gate run with none of them is not the
    gate Make ran: GOCACHE lands outside the checkout, which breaks the
    Unix-socket tests on path length, and an unpinned GOTOOLCHAIN makes
    golangci-lint print "0 issues" and exit non-zero on a cold cache
    (`le/devtools/toolchain.py`).

    This is a DEFAULT rather than a per-area decision because the failure is
    silent and the areas that need something different know it: a race build
    needs CGO_ENABLED=1, a test run needs GOMAXPROCS. Those pass their own
    resolver. An area that passes nothing used to get the ambient environment,
    which is how three `go run` areas ended up running with no pin at all.
    """
    return toolchain().environment()


def action(
    opts: Options,
    gates: GateSet,
    env: Callable[[Gate], dict[str, str] | None] = default_environment,
) -> int:
    """Run what the options select. Returns the process exit code.

    `env` resolves the environment per gate, because one area can hold gates
    with different needs: a race build and a plain build sit side by side in
    `test-unit`.
    """
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
        # Refuse rather than run the plain command. A caller that asked for
        # JSON is going to parse what comes back, and prose on stdout with a
        # zero exit is the worst of the three possible answers: it reads as
        # success and decodes as nothing.
        if not chosen[0].has_json:
            echo(f'{chosen[0].name} has no machine-readable report')
            return 2
        return run_gate(chosen[0], as_json=True, env=env(chosen[0]))

    # The gate's OWN exit code is carried out, not collapsed to 1.
    #
    # `mk/check-rules.mk` warned about exactly this and the port walked into
    # it: the discovery-index check exits 0 for fresh, 3 for STALE and 1 when
    # the generator itself failed, and `scripts/dev/commit_helper.py` BLOCKS on
    # 3 while staying warn-only on 1. Its comment said "do not simplify a
    # caller into one that cannot tell them apart". A caller that answers 1 for
    # every failure is that caller.
    #
    # The FIRST non-zero code wins, because it is the one whose meaning a
    # reader can act on: a later gate's 1 says nothing about the first gate's
    # 3. A sweep still reports every failure by name.
    failed: list[str] = []
    code = 0
    for gate in chosen:
        result = run_gate(gate, env=env(gate))
        if result != 0:
            failed.append(gate.name)
            if code == 0:
                code = result

    echo()
    if failed:
        echo(f'Failed: {", ".join(failed)}')
        return code
    echo(f'{gates.area}: {len(chosen)} gate(s) passed.')
    return 0


def main(
    argv: Sequence[str] | None,
    gates: GateSet,
    doc: str | None = None,
    run: Callable[[Options], int] | None = None,
    env: Callable[[Gate], dict[str, str] | None] = default_environment,
) -> int:
    """Standalone entry for a gate area. Parses, then calls the area's `action`.

    `run` is the MODULE's action, and passing it is not optional for a module
    that defines one. Without it this called `gateapp.action` directly, so a
    module with a custom action had two routes that did different things: the
    dispatcher reached the custom one and `python3 -m le.application.<area>`
    reached the shared one.

    That is the exact divergence `le/registry.py` claims the layout prevents,
    and it was live rather than theoretical. `check_rules` orders its
    generators by WRITE_ORDER because two of them parse what a third writes;
    the standalone route ran them in declaration order instead, which put
    `ze-rules-index-update` before `ze-rules-condensed-update`. The render
    still happened first by accident of declaration order, so nothing broke
    yet, which is what makes it worth naming rather than quietly correcting.

    Defaulting to the shared action keeps the pure-table modules to one line.
    """
    parser = argparse.ArgumentParser(prog=f'le {gates.area}', description=doc)
    add_arguments(parser, gates)
    opts = options(parser.parse_args(argv))
    if run is not None:
        return run(opts)
    return action(opts, gates, env)
