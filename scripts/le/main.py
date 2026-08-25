"""argv routing.

Holds no work of its own. It picks a subprogram out of the registry, lets that
subprogram declare its own flags, and calls its `action`. Adding an area
changes `le/registry.py` and nothing here.

The parse happens ONCE, here, and the result is handed to the subprogram's
`options` to become a typed value. Dispatch then calls `action(options)`
directly -- it never re-enters the subprogram's `main`, which would parse the
same argv a second time and give two routes two chances to disagree.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import __version__
from le.paths import repo_root
from le.registry import REGISTRY, Entry

__all__ = ['build_parser', 'main']

DESCRIPTION = """\
le -- the Ze build and test entry point.

Every subprogram also runs on its own:

    PYTHONPATH=scripts python3 -m le.application.setup --check

which does exactly what `le setup --check` does. Both routes call the same
action with the same typed options, so the two cannot diverge.
"""


def build_parser() -> argparse.ArgumentParser:
    """The top-level parser, with one subparser per registered subprogram.

    Each subprogram's own `add_arguments` fills its subparser, so the flags
    `le setup --check` accepts are declared in `le/application/setup.py` and
    nowhere else.
    """
    parser = argparse.ArgumentParser(
        prog='le',
        description=DESCRIPTION,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument('--version', action='version', version=f'le {__version__}')

    subparsers = parser.add_subparsers(dest='subprogram', metavar='<subprogram>')
    for entry in REGISTRY:
        sub = subparsers.add_parser(entry.name, help=entry.help, description=entry.help)
        entry.load().add_arguments(sub)
        sub.set_defaults(entry=entry)
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    """Route argv to a subprogram's `action`. Returns the process exit code.

    `ZE_REPO_ROOT` is settled and exported FIRST, before any subprogram loads.
    Every gate then inherits one root rather than each rediscovering it, and a
    gate that shells out passes it on. A tool run without `le` discovers the
    same answer for itself (`le/paths.py`), so the variable makes the root
    explicit rather than mandatory.
    """
    repo_root(export=True)

    parser = build_parser()
    args = parser.parse_args(argv)

    if args.subprogram is None:
        parser.print_help()
        return 1

    entry: Entry = args.entry
    module = entry.load()
    return module.action(module.options(args))


if __name__ == '__main__':
    sys.exit(main())
