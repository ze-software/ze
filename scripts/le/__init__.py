"""le -- the Ze build and test entry point.

A typed Python replacement for the Makefile. It is being built beside the
Makefile rather than in place of it: both work, and the Makefile is removed
only once every target has a counterpart here and the parity gate agrees.

Layout, and the reason for it:

    le/application/  one module per subprogram, mirroring one mk/*.mk area
    le/devtools/     libraries the subprograms share
    le/main.py       argv routing

Every subprogram module exposes the same three names, so the dispatcher needs
to know nothing about any of them:

    add_arguments(parser)   the flags, declared once and used by both routes
    action(options)         the work: typed, importable, and what dispatch calls
    main(argv)              a standalone shell, so the module runs on its own

Dispatch calls `action` directly. `main` exists only so that

    PYTHONPATH=scripts python3 -m le.application.setup --check

does the same thing as

    ./le setup --check

with no logic written twice.
"""

from __future__ import annotations

__all__ = ['__version__']

__version__ = '0.1.0'
