"""Every gate `le` knows, across every area, as data.

    ./le gates              one line per gate
    ./le gates --json       the same, machine-readable
    ./le gates --writes     only the generators
    ./le gates --area rules only one area

**This exists because the port blinded a guard.**
`TestRegenCheckReadonlyCoversGenerators` (`scripts/status/verify_run_test.go`)
asks one question worth asking: does every generator reachable from `make
generate` or `ze-generated-files-update` have a read-only check guarding its
output? It answered it by parsing the RECIPE TEXT of those targets for a
`.py` or `.go` producer.

That worked while a recipe named its script. A recipe that delegates to `le`
names none, so the producer simply vanished from the derived population and the
loop that checks each one never saw it. `rules_points.py`, `rules_index.py`,
`rules_condensed.py`, `package_map.py`, `docs_to_code.py`, `code_to_docs.py`,
`arch_map.py` and `testing_health.py` all dropped out this way. Nothing went
red. The coverage was simply gone, which is the failure mode this repository
keeps a journal class for.

Recipe text was always a proxy for the question. The gate table is the answer
itself, so the guard reads this instead and a delegating recipe stops hiding
anything.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Sequence
from dataclasses import dataclass

from le.console import echo
from le.registry import REGISTRY

__all__ = ['Options', 'action', 'add_arguments', 'catalogue', 'main', 'options']


@dataclass(frozen=True)
class Options:
    """Everything `action` needs, and nothing about how it was asked for."""

    as_json: bool = False
    writes_only: bool = False
    area: str = ''


def add_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument('--json', dest='as_json', action='store_true', help='machine-readable')
    parser.add_argument(
        '--writes',
        dest='writes_only',
        action='store_true',
        help='only the gates that regenerate a file',
    )
    parser.add_argument('--area', default='', help='only this area')


def options(namespace: argparse.Namespace) -> Options:
    return Options(
        as_json=bool(namespace.as_json),
        writes_only=bool(namespace.writes_only),
        area=str(namespace.area),
    )


def catalogue(*, writes_only: bool = False, area: str = '') -> list[dict[str, object]]:
    """Every gate, as plain records.

    The `argv` is the whole command, so a reader can see which script a gate
    runs without importing anything of `le`'s. That is what the Go guard needs
    and it is what a human debugging a gate wants too.
    """
    found: list[dict[str, object]] = []
    for entry in REGISTRY:
        if area and entry.name != area:
            continue
        module = entry.load()
        gates = getattr(module, 'GATES', None)
        if gates is None:
            continue  # setup and lint are not gate areas
        for gate in gates.gates:
            if writes_only and not gate.writes:
                continue
            found.append(
                {
                    'area': entry.name,
                    'name': gate.name,
                    'argv': list(gate.argv),
                    'script': gate.python_script or '',
                    'writes': gate.writes,
                    'json': gate.has_json,
                    'why': gate.why,
                }
            )
    return found


def action(opts: Options) -> int:
    """Print the catalogue. Returns the process exit code."""
    found = catalogue(writes_only=opts.writes_only, area=opts.area)

    if opts.area and not found:
        echo(f'no such area: {opts.area}')
        echo(f'try one of: {", ".join(e.name for e in REGISTRY)}')
        return 2

    if opts.as_json:
        # Nothing but the document on stdout: a caller asked for JSON and is
        # going to parse what comes back.
        print(json.dumps(found, indent=2, sort_keys=True))
        return 0

    width = max((len(str(row['name'])) for row in found), default=0)
    for row in found:
        mark = 'writes' if row['writes'] else 'checks'
        echo(f'  {row["name"]!s:<{width}}  {mark}  {row["area"]}')
    echo()
    echo(f'{len(found)} gate(s) across {len({row["area"] for row in found})} area(s)')
    return 0


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog='le gates', description=__doc__)
    add_arguments(parser)
    return action(options(parser.parse_args(argv)))


if __name__ == '__main__':
    sys.exit(main())
