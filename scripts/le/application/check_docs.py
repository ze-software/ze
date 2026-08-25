"""Documentation gates: drift, wiring, anchors, indexes, and Simplified Technical English.

Ported from mk/check-docs.mk.

    ./le check-docs                       every check in the area
    ./le check-docs --list                what each one is for
    ./le check-docs ze-digest-check       one of them
    ./le check-docs ze-ste-review --json

Three targets did NOT move and are still recipes in mk/check-docs.mk, each
because it is a shell program rather than a command:

    ze-doc-verify           a twelve-step sequence over one FAIL flag, ending in
                            a conditional exit. A Gate is one command.
    ze-wiki-commands-update a pipeline into a redirect, writing outside the
                            checkout (../wiki/command-catalog.md).
    ze-wiki-update          an alias whose only content is a prerequisite edge.

`ze-ste-check` lives here and is deliberately NOT part of ze-doc-verify. It
counts the six banned habits per changed file against that file's own HEAD
version, and several sessions share this checkout, so a tree-wide run reports a
sibling session's in-flight sentences with no way to tell whose they are. The
blocking gate is in commit_helper.py, scoped to one commit's files.
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

# `$(MAKE)` cannot be recovered here: make exports GOCACHE and the other
# `export`ed variables to a recipe's environment, and MAKE is not one of them.
# `make` is what verify_wiring_docs.py itself defaults to (its `--make` flag),
# so passing it names the same executable the recipe named.
_MAKE = 'make'


def _py(script: str, *args: str) -> tuple[str, ...]:
    return ('python3', f'scripts/dev/{script}', *args)


GATES = GateSet(
    area='check-docs',
    gates=(
        Gate(
            name='ze-spec-citation-check',
            argv=_py('spec-citation-check.py'),
            why=(
                'a plan/spec-*.md citing a sibling spec absent on disk fails, unless the'
                ' target is grandfathered in plan/.citation-baseline; a path:line citation'
                ' whose backtick-quoted token drifted off that line warns'
            ),
        ),
        Gate(
            name='ze-doc-drift-check',
            argv=tuple(_go('scripts/docvalid/doc_drift.go')),
            why='documentation claims agree with the live registry, the Makefile and the filesystem',
        ),
        Gate(
            name='ze-doc-wiring-check',
            argv=_py('verify_wiring_docs.py', '--make', _MAKE),
            why=(
                'changed-file-aware wiring, documentation, command and inventory gate;'
                ' ze-precommit-verify runs it, and it routes ze-spec-citation-check when'
                ' a plan/ file changed'
            ),
        ),
        Gate(
            name='ze-doc-index-check',
            argv=_py('code_to_docs.py', '--check'),
            why='every `<!-- source: -->` anchor in docs/ resolves to a real file and symbol',
        ),
        Gate(
            name='ze-digest-check',
            argv=_py('digest_check.py'),
            why=(
                'every file:line reference in ai/digests/*.md resolves to a real file and an'
                ' in-range line; the digests are hand-maintained, so this catches the anchors'
                ' rotting when code moves'
            ),
        ),
        Gate(
            name='ze-consistency-check',
            argv=('go', 'run', 'scripts/lint/consistency.go', '.'),
            why='code and documentation agree on design references, cross-references and stale references',
        ),
        Gate(
            name='ze-ste-check',
            argv=_py('ste_check.py', '--check'),
            why=(
                'no ASD-STE100 habit grew against HEAD in a changed file. HEAD is the'
                ' baseline, so legacy prose stays until someone rewrites it, no baseline'
                ' file exists to re-bless, and the one way to green is to fix the prose'
            ),
        ),
        Gate(
            name='ze-ste-review',
            argv=_py('ste_check.py'),
            json_flag='--json',
            why='every ASD-STE100 finding in the tree, with file:line and the fix',
        ),
        Gate(
            name='ze-ste-review-changed',
            argv=_py('ste_check.py', '--changed'),
            why='the same findings, over the files this working tree changed',
        ),
        Gate(
            name='ze-doc-index-update',
            argv=_py('code_to_docs.py'),
            why='regenerate ai/CODE-TO-DOCS.md, the source-to-document reverse index',
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
