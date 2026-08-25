"""Verify helpers: what the working tree holds, and the throwaway-worktree gate run.

Ported from mk/helper-verify.mk.

    ./le helper-verify --list                     what each one is for
    ./le helper-verify ze-working-tree-check      changed paths, grouped by area
    ./le helper-verify ze-verify-worktree         the gate, against a COMMIT

This area names no aggregate run, and `action` refuses one. Every other area
runs its whole set when given no gate name, which is what `make ze-<area>-check`
did. Here the set is a two-second advisory and a 25-to-53-minute gate, so the
convenient spelling would be the expensive one, and no Make target ever meant
"both".

MAX_AREAS, COMMIT and KEEP moved with the targets, from `$(if ...)` to the
environment. A variable set on the make command line reaches the recipe's
environment, so `make ze-verify-worktree COMMIT=abc123 KEEP=1` and
`COMMIT=abc123 KEEP=1 ./le helper-verify ze-verify-worktree` build the same argv.
"""

from __future__ import annotations

import argparse
import os
import sys
from collections.abc import Sequence

from le import gateapp
from le.console import echo
from le.devtools.gate import Gate, GateSet

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']


def _py(script: str, *args: str) -> tuple[str, ...]:
    return ('python3', f'scripts/dev/{script}', *args)


def _working_tree_argv() -> tuple[str, ...]:
    """`working_tree_check.py`, made a gate when MAX_AREAS names a ceiling."""
    ceiling = os.environ.get('MAX_AREAS', '').strip()
    if ceiling:
        return _py('working_tree_check.py', '--max-areas', ceiling)
    return _py('working_tree_check.py')


def _verify_worktree_argv() -> tuple[str, ...]:
    """`verify_worktree.py` over the commit asked for, keeping the tree if asked."""
    args: list[str] = []
    commit = os.environ.get('COMMIT', '').strip()
    if commit:
        args.extend(('--commit', commit))
    if os.environ.get('KEEP', '').strip():
        args.append('--keep')
    return _py('verify_worktree.py', *args)


GATES = GateSet(
    area='helper-verify',
    gates=(
        Gate(
            name='ze-working-tree-check',
            argv=_working_tree_argv(),
            why=(
                'how wide the uncommitted tree is, grouped by area. Advisory: it reports and'
                ' exits 0, because only a person can say whether two areas are one logical'
                ' change. The failure it exists to surface is several FINISHED chunks held in'
                ' one tree, which a checkout destroys and which every later chunk must be'
                ' diffed around. MAX_AREAS=N makes it a gate'
            ),
        ),
        Gate(
            name='ze-verify-worktree',
            argv=_verify_worktree_argv(),
            why=(
                'run the pre-commit gate against a COMMIT, in a throwaway worktree, so the'
                ' working tree stays free and no mid-run edit can invalidate the result.'
                ' 25-53 minutes on this hardware. COMMIT=<rev> picks the commit (default'
                ' HEAD), KEEP=1 leaves the worktree'
            ),
        ),
    ),
)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select, and refuse the bare area run.

    A caller who types `./le helper-verify` wants the cheap advisory far more
    often than the hour-long gate, and the shared helper would start both.
    """
    if not opts.names and not opts.listing:
        echo('helper-verify has no aggregate run: name the gate you want.')
        echo(f'  {", ".join(GATES.names())}')
        return 2
    return gateapp.action(opts, GATES)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
