"""Scratch-tree gates: the tmp/ and cache/ symlinks a run writes through.

Ported from the root Makefile.

    ./le scratch --list                        what each one is for
    ./le scratch --write                       both generators
    ./le scratch ze-scratch-links-ensure       one of them

Both gates WRITE, so the area declares no check and a bare `./le scratch` says
so. That is the honest reading: neither one reports on the tree, and
`ze-scratch-links-ensure` is a prerequisite edge in mk/test-unit.mk rather than
something a sweep should discover.

**The six destructive targets stayed in the root Makefile, and the reason is
the same for all of them: each is a shell PROGRAM whose guards are the safety.**
`ai/rules/never-destroy-work.md` governs them, and a guard rewritten in Python
is a guard re-derived rather than moved.

    ze-scratch-clean    two `find` sweeps whose -mindepth, -not -name and
                        -mmin bounds are what keep `rm -rf` off tmp/ itself,
                        tmp/session/ and tmp/kernel/, plus a `$$(...)` count
    ze-session-clean    an `if`, a `case` date check, and a `for` loop, with
                        BEFORE reaching the recipe through the ENVIRONMENT
                        rather than spliced into the shell text
    ze-session-reap     `$(if $(DRY),--dry-run)`, a Make conditional in argv
    clean               a `$(MAKE)` re-entry and `env -u GOCACHE`
    clean-all           an `if [ -e tmp ]`, a `find -exec`, and `env -u GOCACHE`
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']


def _links(*args: str) -> tuple[str, ...]:
    return ('python3', 'scripts/dev/ensure-links.py', *args)


GATES = GateSet(
    area='scratch',
    gates=(
        Gate(
            name='ze-scratch-links-ensure',
            argv=_links('--quiet'),
            why=(
                'point the tmp/ and cache/ symlinks at their out-of-tree targets before any'
                ' target writes scratch. This replaces the old tmp/go.mod nested-module'
                ' sentinel: `go list ./...` skips a directory SYMLINK named tmp/ (verified),'
                ' so no marker file is needed (plan/spec-relocate-scratch-and-cache.md)'
            ),
            writes=True,
        ),
        Gate(
            name='ze-scratch-migrate',
            argv=_links('--migrate'),
            why=(
                'the same cutover for a checkout whose tmp/ or cache/ is still a REAL'
                ' directory: move its entries to the out-of-tree target and leave a symlink'
                ' behind, refusing rather than clobbering a name the target already holds'
                ' (scripts/dev/ensure-links.py, migrate). A path that is already a symlink'
                ' needs no migration and takes the ensure route instead'
            ),
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
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
