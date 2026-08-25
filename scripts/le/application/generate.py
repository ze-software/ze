"""Code generators and the freshness gates that hold their outputs current.

Ported from the root Makefile.

    ./le generate                            every check
    ./le generate --write                    every generator
    ./le generate --list                     what each gate is for
    ./le generate ze-web-assets-check        one of them

Each generator writes a tracked file, and each has a `--check` twin that
regenerates in memory and diffs instead. The pairs are what makes a generated
file gateable: `ze-generated-files-check` in the Makefile is nothing but a list
of the check halves, so a generator that gains no check is a file that can go
stale with every gate green.

THE `generate:` RECIPE ITSELF DID NOT MOVE, and neither did the two templ
targets. Two Go tests in scripts/status/verify_run_test.go DERIVE their subject
from the literal recipe text of those three:

    TestRegenCheckReadonlyCoversGenerators   reads the six generator scripts out
                                             of `generate:` and fails when one
                                             gains no read-only check
    TestTemplCheckIsReadOnlyAndReportsOrphans
                                             reads the templ call sites out of
                                             `generate:` and ze-templ-output-check,
                                             and fails when a -check call loses
                                             -keep-orphaned-files

A delegating one-liner leaves both with an empty population, so the recipes stay
where the guards can read them. The four `--check` twins below are unaffected:
each is its own target with its own command.

The vendor sync IS a generator, and it is here for the reason the rest are: a
consumer asset copy is a generated file. `//go:embed` cannot reach outside its
own package, so one library is vendored once per consumer, and a copy nothing
regenerates diverges from `third_party/web/` without a sound.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']


def _go(script: str, *args: str) -> tuple[str, ...]:
    """`go run <script>`, argument for argument.

    NO tag flag, which is what these recipes spelled. `toolchain().go_run` adds
    the full feature tag set and is a different command; the codegen scripts
    carry `//go:build ignore` and take none.
    """
    return ('go', 'run', script, *args)


def _py(script: str, *args: str) -> tuple[str, ...]:
    return ('python3', f'scripts/dev/{script}', *args)


GATES = GateSet(
    area='generate',
    gates=(
        Gate(
            name='ze-yang-glue-check',
            argv=_go('scripts/codegen/yang_glue.go', '--check'),
            why='the generated yang/*/register.go and embed.go agree with the .yang tree',
        ),
        Gate(
            name='ze-plugin-imports-check',
            argv=_go('scripts/codegen/plugin_imports.go', '--check'),
            why=(
                'the blank imports in internal/component/plugin/all/all.go are current, so'
                ' the composition root still registers every plugin the tree holds'
            ),
        ),
        Gate(
            name='ze-feature-tags-check',
            argv=_go('scripts/codegen/feature_tags.go', '--check'),
            why=(
                'the three files derived from feature-gates.txt are current: .golangci.yml,'
                ' gokrazy/ze/config.json and docs/guide/quickstart.md'
            ),
        ),
        Gate(
            name='ze-web-assets-check',
            argv=_go('scripts/codegen/web_assets.go', '--check'),
            why=(
                'each page_assets.go agrees with the markup its package renders. The'
                ' generator walks the templ component graph from each page, so a component'
                ' that gains hx-ext="sse" changes the set for every page reaching it. A'
                ' stale file leaves a page missing an extension it now needs, which is'
                ' invisible everywhere but the browser: the page renders and does nothing'
            ),
        ),
        Gate(
            name='ze-vendor-web-check',
            argv=_go('scripts/vendor/check_web.go'),
            why=(
                'each consumer asset copy matches third_party/web/. It reads two directory'
                ' trees and no network, so it runs in an offline CI and an offline checkout'
            ),
        ),
        Gate(
            name='ze-arch-map-check',
            argv=_py('arch_map.py', '--check'),
            why='the architecture lists in ai/INSTRUCTIONS.md are current with the tree',
        ),
        Gate(
            name='ze-htmx-upgrade-check',
            argv=_py('htmx_upgrade_check.py', '--check'),
            why=(
                "htmx's OWN scanner, vendored at third_party/web/htmx-upgrade-check.py,"
                ' reports no htmx 4 issue that scripts/dev/htmx-upgrade-explained.txt does'
                ' not account for. It builds a DOM, so it reads the inheritance carriers a'
                ' text search cannot'
            ),
        ),
        Gate(
            name='ze-htmx-upgrade-report',
            argv=_py('htmx_upgrade_check.py', '--report'),
            why='print every htmx 4 upgrade issue, explained or not, and exit 0',
        ),
        Gate(
            name='ze-vendor-web-update-report',
            argv=_go('scripts/vendor/check_web.go', '--updates'),
            why=(
                'ask the npm registry for newer versions of the vendored web assets. This'
                ' is where the network query lives, and it is why ze-vendor-web-check has'
                ' none'
            ),
        ),
        Gate(
            name='ze-vendor-web-sync',
            argv=_go('scripts/vendor/sync_web.go'),
            why='copy third_party/web/ into each consumer package that embeds it',
            writes=True,
        ),
        Gate(
            name='ze-arch-map-update',
            argv=_py('arch_map.py'),
            why='regenerate the architecture lists in ai/INSTRUCTIONS.md',
            writes=True,
        ),
    ),
)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select.

    The two generators here write different files and read none of each
    other's output, so `--write` needs no order of its own.
    """
    return gateapp.action(opts, GATES)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
