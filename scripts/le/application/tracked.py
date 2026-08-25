"""Does `le` still work from what git holds?

    ./le tracked              check HEAD
    ./le tracked --rev <rev>  check any commit

The working tree and the committed tree are two different trees, and only one
of them is what anybody else gets. This asks about the second.

Run it after any commit touching `scripts/le`, the way
`ze-repository-tracked-build-check` is run after a commit carrying Go. That
check compiles what git holds, and it is why Go has never had the failure this
one exists for: on 2026-08-25 a clean archive of HEAD failed to load 21 of 21
areas while the working tree ran perfectly.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence
from dataclasses import dataclass

from le.console import echo
from le.devtools.tracked import check_tracked_import

__all__ = ['Options', 'action', 'add_arguments', 'main', 'options']


@dataclass(frozen=True)
class Options:
    """Everything `action` needs, and nothing about how it was asked for."""

    revision: str = 'HEAD'


def add_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument(
        '--rev',
        dest='revision',
        default='HEAD',
        help='the commit to check (default HEAD)',
    )


def options(namespace: argparse.Namespace) -> Options:
    return Options(revision=str(namespace.revision))


def action(opts: Options) -> int:
    """Load every registered area from the committed tree. Returns the exit code."""
    echo(f'==> loading every le area from {opts.revision}')
    verdict = check_tracked_import(opts.revision)

    if verdict.detail:
        echo(f'  {verdict.detail}')
        return 2

    if not verdict.ok:
        for line in verdict.broken:
            echo(f'  BROKEN  {line}')
        echo()
        echo(f'{len(verdict.broken)} of {verdict.areas} area(s) do not load from {opts.revision}.')
        echo('The working tree is not the tree anybody else gets. A file present here')
        echo('and absent from the commit is exactly what this finds.')
        return 1

    echo(f'  {verdict.areas} area(s) load from {opts.revision}')
    return 0


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog='le tracked', description=__doc__)
    add_arguments(parser)
    return action(options(parser.parse_args(argv)))


if __name__ == '__main__':
    sys.exit(main())
