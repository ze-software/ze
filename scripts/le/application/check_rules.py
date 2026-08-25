"""Rules-system gates: the point corpus, its renders, and the discovery indexes.

Ported from mk/check-rules.mk.

    ./le check-rules                            every check
    ./le check-rules --write                    every generator
    ./le check-rules ze-rules-render-check      one of them

**Order is load-bearing here, and it is why `--write` runs a fixed sequence
rather than whatever the caller asked for.** `rules_index.py` and
`rules_condensed.py` both parse the RENDERED rules, which `rules_points.py
render` writes with a plain write_text. In Make that ordering was expressed as
a prerequisite edge, because `make -j` honours prerequisite order and not
recipe order: without the edge a digest could be built from pre-render text or
from a torn read. Here the sequence is a list, and `run_writers` walks it.

The render check does NOT subsume the round trip. They read the two directions
of one identity. `render --check` asks whether the rendered rule matches the
points; the round trip asks whether the rendered rule can be split back into
points at all. One blank line at the top of a point body satisfies the first
and breaks the second, and the corpus is then permanently un-splittable with
every other gate green.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.console import echo
from le.devtools.gate import Gate, GateSet, run_all, run_gate

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']


def _py(script: str, *args: str) -> tuple[str, ...]:
    return ('python3', f'scripts/dev/{script}', *args)


GATES = GateSet(
    area='check-rules',
    gates=(
        Gate(
            name='ze-rules-render-check',
            argv=_py('rules_points.py', 'render', '--check'),
            why='the rendered ai/rules/*.md agree with ai/rules/points/',
        ),
        Gate(
            name='ze-rules-points-roundtrip-check',
            argv=_py('rules_points.py', 'roundtrip'),
            why=(
                'every rendered rule can be split back into points byte-identically;'
                ' a lossy split is silent instruction loss'
            ),
        ),
        Gate(
            name='ze-rules-index-check',
            argv=_py('rules_index.py', '--check'),
            why='ai/rules/INDEX.md lists every rule',
        ),
        Gate(
            name='ze-rules-condensed-check',
            argv=_py('rules_condensed.py', '--check'),
            why='TRIGGERS.md and CORE.md are current with the rendered rules',
        ),
        Gate(
            name='ze-rules-lint',
            argv=_py('rules_lint.py'),
            why=(
                'every rule carries the **When:** / **Severity:** metadata block'
                ' (ai/rules/rule-format.md), so tooling parses triggers rather'
                ' than guessing them'
            ),
        ),
        Gate(
            name='ze-discovery-index-check',
            argv=_py('package_map.py', '--check'),
            why='ai/PACKAGE-MAP.md is current with the tree',
        ),
        Gate(
            name='ze-rules-render-update',
            argv=_py('rules_points.py', 'render'),
            why='render ai/rules/points/ into ai/rules/*.md',
            writes=True,
        ),
        Gate(
            name='ze-rules-index-update',
            argv=_py('rules_index.py'),
            why='regenerate ai/rules/INDEX.md from the rendered rules',
            writes=True,
        ),
        Gate(
            name='ze-rules-condensed-update',
            argv=_py('rules_condensed.py'),
            why='regenerate TRIGGERS.md and CORE.md from the rendered rules',
            writes=True,
        ),
        Gate(
            name='ze-discovery-index-update',
            argv=_py('package_map.py'),
            why='regenerate ai/PACKAGE-MAP.md',
            writes=True,
        ),
        Gate(
            name='ze-docs-to-code-update',
            argv=_py('docs_to_code.py'),
            why='regenerate ai/DOCS-TO-CODE.md',
            writes=True,
        ),
    ),
)

# The generators, in the ONE order that is correct. The render must complete
# before either digest parses its output.
WRITE_ORDER: tuple[str, ...] = (
    'ze-rules-render-update',
    'ze-rules-condensed-update',
    'ze-rules-index-update',
    'ze-discovery-index-update',
    'ze-docs-to-code-update',
)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select.

    `--write` with no names is the one case this area cannot delegate: the
    generators have a required order, and the shared helper would run them in
    declaration order without knowing that the digests read the render's
    output.
    """
    if opts.write and not opts.names and not opts.listing:
        ordered = [gate for name in WRITE_ORDER if (gate := GATES.find(name)) is not None]
        failed = run_all(ordered)
        echo()
        if failed:
            echo(f'Failed: {", ".join(failed)}')
            return 1
        echo(f'check-rules: {len(ordered)} generator(s) ran, render first.')
        return 0
    return gateapp.action(opts, GATES)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

__all__ = [*__all__, 'WRITE_ORDER', 'run_gate']

if __name__ == '__main__':
    sys.exit(main())
