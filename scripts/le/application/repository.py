"""Structural gates: where code may live, what it may call, and what git holds.

Ported from the root Makefile.

    ./le repository                          every check, cheapest first
    ./le repository --list                   what each gate is for
    ./le repository ze-tier-check            one of them
    ./le repository ze-ci-dispatch-check --json

Almost every gate here ships with a SELFTEST, and the selftest is a separate
gate rather than a hidden first step. A structural check is an AST scan whose
green is indistinguishable from a scan that matched nothing, so each one proves
itself against fixtures whose answer is known before it judges the live tree.
The Make targets ran the pair as two lines of one recipe; here each half has a
name and can be run on its own, which it could not be as half a recipe.

The Makefile target that ran a pair now names both gates in one `le` call, so
the order is still the recipe's order.

REV and ARGS moved with the targets, from `$(if ...)` to the environment. A
variable set on the make command line reaches the recipe's environment, so
`make ze-repository-tracked-build-check REV=7abe8a07e` and
`REV=7abe8a07e ./le repository ze-repository-tracked-build-check` build the
same argv.

TWO GATES ARE EXPENSIVE and sit last in the list for that reason:
ze-staticcheck-feature-matrix-check type-checks the tree once per feature-tag
configuration, and ze-repository-tracked-build-check extracts a commit and
compiles it (about 45 seconds).
"""

from __future__ import annotations

import argparse
import os
import sys
from collections.abc import Sequence

from le import gateapp
from le.console import echo
from le.devtools.gate import Gate, GateSet
from le.devtools.toolchain import toolchain

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']

# The two non-Linux targets ze-platform-vet vetted, and the GOOS each gate
# needs. Declared rather than derived from the gate name, so a third platform
# is one row here and one Gate below.
PLATFORMS: dict[str, str] = {
    'ze-platform-vet-darwin': 'darwin',
    'ze-platform-vet-freebsd': 'freebsd',
}

# The trees that ship a non-Linux stub, and the only packages ze-platform-vet
# ever vetted.
PLATFORM_PACKAGES: tuple[str, ...] = (
    './internal/component/host/...',
    './internal/component/iface/...',
    './internal/plugins/iface/...',
)


def _go(script: str, *args: str) -> tuple[str, ...]:
    """`$(GO) run <script>`, argument for argument.

    Plain `go run` with NO tag flag, which is what these recipes spelled. The
    check programs carry `//go:build ignore` and take none.

    `$(GO)` was `GO ?= go`, so an operator naming another toolchain reached
    these gates through the environment. Reading GO here keeps that route:
    make places a command-line variable in the recipe's environment, so
    `make ze-tier-check GO=/path/to/go` still runs the go it names.
    """
    return (os.environ.get('GO', 'go'), 'run', script, *args)


def _py(script: str, *args: str) -> tuple[str, ...]:
    return ('python3', f'scripts/dev/{script}', *args)


def _words(name: str) -> tuple[str, ...]:
    """An environment variable read the way make expanded `$(VAR)` into a recipe.

    Unset and empty both yield no argument at all, and a value is split on
    whitespace, which is what the shell did to the expansion.
    """
    return tuple(os.environ.get(name, '').split())


def _tracked_build_argv() -> tuple[str, ...]:
    """`tracked_build.go` over the commit asked for, plus the caller's own flags."""
    rev = os.environ.get('REV', '').strip()
    revision = (f'--rev={rev}',) if rev else ()
    return (*_go('scripts/checks/tracked_build.go'), *revision, *_words('ARGS'))


GATES = GateSet(
    area='repository',
    gates=(
        Gate(
            name='ze-tier-selftest',
            argv=_py('dep_audit.py', '--selftest'),
            why=(
                "the tier gate's own isolated fixtures -- engine placement, and the"
                ' wired-versus-core classification -- before it judges the live tree'
            ),
        ),
        Gate(
            name='ze-tier-check',
            argv=_py('dep_audit.py', '--check'),
            why=(
                'module-tier placement (ai/rules/architecture.md): a config-driven engine'
                ' lives in internal/component/ when a feature depends on it, and in'
                ' internal/plugins/ otherwise'
            ),
        ),
        Gate(
            name='ze-protocol-skeleton-report',
            argv=_py('protocol_skeleton_report.py'),
            why='which protocol implementations are still a skeleton rather than a daemon',
        ),
        Gate(
            name='ze-iface-resolution-check',
            argv=_go('scripts/checks/iface_resolution.go'),
            why=(
                'interface consumers resolve a logical name through the shared iface'
                ' resolver, never the kernel directly. scripts/checks/iface_resolution.go'
                ' owns the allowlist of legitimate direct-resolution sites'
            ),
        ),
        Gate(
            name='ze-plugin-boundary-selftest',
            argv=_go('scripts/checks/plugin_process_boundary.go', '--selftest'),
            why='the dangerous-pattern detector fires on its fixtures before it scans',
        ),
        Gate(
            name='ze-plugin-boundary-check',
            argv=_go('scripts/checks/plugin_process_boundary.go'),
            why=(
                'plugin process boundary (ai/rules/plugins.md): a plugin calling another'
                " in-process package's same-process-effect function directly, rather than"
                ' through DirectBridge or DispatchCommand, guards with IsInternal() so it'
                ' does not silently no-op when it runs as an external subprocess'
            ),
        ),
        Gate(
            name='ze-config-coercion-selftest',
            argv=_go('scripts/checks/config_string_coercion.go', '--selftest'),
            why='the coercion detector fires on its fixtures before it scans',
        ),
        Gate(
            name='ze-config-coercion-check',
            argv=_go('scripts/checks/config_string_coercion.go'),
            why=(
                'the framework delivers YANG leaf values as JSON strings, so a config.go'
                ' coercing one with a native-type assertion silently ignores the value and'
                ' reverts to the default. A bool `enabled` gate then disables the feature,'
                ' which is how ddos-detect never fired'
            ),
        ),
        Gate(
            name='ze-fs-persistence-selftest',
            argv=_go('scripts/checks/direct_fs_persistence.go', '--selftest'),
            why='the direct-filesystem detector fires on its fixtures before it scans',
        ),
        Gate(
            name='ze-fs-persistence-check',
            argv=_go('scripts/checks/direct_fs_persistence.go'),
            why='persistent state is written through the storage component, not by a raw file call',
        ),
        Gate(
            name='ze-dash-stdio-selftest',
            argv=_go('scripts/checks/cli_dash_stdio.go', '--selftest'),
            why='the AST taint detector fires on the pre-migration shapes before the live scan',
        ),
        Gate(
            name='ze-dash-stdio-check',
            argv=_go('scripts/checks/cli_dash_stdio.go'),
            why=(
                'a filename-accepting command reads and writes a user-supplied path through'
                ' internal/core/cliio, so "-" means stdin and stdout, never through a raw os call'
            ),
        ),
        Gate(
            name='ze-port-defaults-selftest',
            argv=_go('scripts/checks/port_defaults.go', '--selftest'),
            why='the port-default mapping proves itself on fixtures before it reads the tree',
        ),
        Gate(
            name='ze-port-defaults-check',
            argv=_go('scripts/checks/port_defaults.go'),
            why=(
                'the hand-maintained table in internal/component/config/listener_defaults.go'
                " matches each service's YANG `refine port { default N }`, because the YANG"
                ' compiler does not propagate a refine default'
            ),
        ),
        Gate(
            name='ze-ci-dispatch-selftest',
            argv=tuple(toolchain().go_run('scripts/checks/ci_dispatch_commands.go', '--selftest')),
            why='the call-site scanner fires on its fixtures before it reads the registry',
        ),
        Gate(
            name='ze-ci-dispatch-check',
            argv=tuple(toolchain().go_run('scripts/checks/ci_dispatch_commands.go')),
            json_flag='--json',
            why=(
                'every command string the repository SENDS still resolves. It runs with the'
                ' full feature tag set, because it enumerates the live command registry: a'
                " reduced set leaves a gated plugin's commands absent and reports every use"
                ' of them as dead'
            ),
        ),
        Gate(
            name='ze-yang-leaf-mentions-selftest',
            argv=_go('scripts/checks/yang_leaf_mentions.go', '--selftest'),
            why='the mention scan fires on a fixture whose answer is known before it reports',
        ),
        Gate(
            name='ze-yang-leaf-mentions-report',
            argv=_go('scripts/checks/yang_leaf_mentions.go'),
            json_flag='--json',
            why=(
                'a YANG leaf whose name appears in no string literal of the owning package'
                ' is PROBABLY never read. ADVISORY and in no verify stage: the signal is a'
                ' heuristic, so it reports and exits 0. The blocking half is the root-claim'
                ' gate in internal/component/plugin/all/config_claims_test.go'
            ),
        ),
        Gate(
            name='ze-test-sensitivity-selftest',
            argv=_go('scripts/checks/inert_tests.go', '--selftest'),
            why='both AST detectors fire on known-bad fixtures before the live counts are read',
        ),
        Gate(
            name='ze-test-sensitivity-check',
            argv=_go('scripts/checks/inert_tests.go', '--check'),
            why=(
                'a test that cannot fail, and a test file no build tag reaches, both read as'
                ' coverage while providing none. Neither is visible in any count of tests,'
                ' which is why the published totals grew for years without this. The counts'
                ' may only go DOWN, and the floors in test/health/sensitivity-baseline.json'
                ' are lowered in the same change that improves the number'
            ),
        ),
        Gate(
            name='ze-test-weakened-selftest',
            argv=_py('check_weakened_tests.py', '--selftest'),
            why=(
                'on a fixture repository whose answer is known, the checker still refuses a'
                ' weakening with no row and accepts the same weakening once a row names it'
            ),
        ),
        Gate(
            name='ze-test-weakened-check',
            argv=_py('check_weakened_tests.py'),
            why=(
                'test/weakened.md still parses for the commit gate. Whether a commit is'
                " covered is a question about THAT commit's paths and a verify stage has"
                ' none, so this checks the one thing true for every session in a shared'
                ' checkout: a header that drifted would leave commit_helper.py reading no rows'
            ),
        ),
        Gate(
            name='ze-repository-check',
            argv=_py('validate.py', '--root', '.'),
            why=(
                'all five repository checks over your own tree: source anchors, cross-package'
                ' wiring, CLI handler coverage and spec AC completeness (~0.2s)'
            ),
        ),
        Gate(
            name='ze-repository-tree-check',
            argv=_py('validate.py', '--root', '.', '--changed-file', ''),
            why=(
                'the three TREE-WIDE checks of ze-repository-check, which is what'
                ' ze-precommit-verify runs. Declaring an EMPTY changed set is what selects'
                ' them: the two changed-file checks hold their subject to a completeness'
                ' standard a half-written file cannot meet, and several sessions share this'
                ' checkout, so inside verify they would red a run whose author changed none'
                ' of it'
            ),
        ),
        Gate(
            name='ze-test-health-check',
            argv=_py('testing_health.py', '--check'),
            why=(
                'docs/features/test-health.md and test/health/latest.json are current.'
                ' Their output is a pure function of committed state, with no wall-clock'
                ' value in it, so staleness is gateable the way every other generated file is'
            ),
        ),
        # The two gates below are the first here whose implementation is the GO
        # `le` rather than a script. The regeneration and the check share ONE
        # derivation (letools/sitefacts, derive), so a Python re-implementation
        # would be a second counter over one tree -- and two counters over one
        # tree drift by construction: the site and the repository disagreed by
        # 30 tests the moment both counted for themselves. When the Makefile
        # routes to the Go `le` directly, these two rows go and the shims stay.
        Gate(
            name='ze-site-facts-update',
            argv=_go('./cmd/le', 'site-facts', 'update'),
            why=(
                'derive the numbers the website publishes about this repository into'
                ' website/data/repo-facts.json, so a site build reads a commit instead of'
                ' walking a checkout several sessions share'
            ),
            writes=True,
        ),
        Gate(
            name='ze-site-facts-check',
            argv=_go('./cmd/le', 'site-facts', 'check'),
            why=(
                'website/data/repo-facts.json still publishes what the last COMMIT says.'
                ' It judges a commit in a throwaway worktree rather than the working tree,'
                ' because a check reading the tree answers differently in two sessions of'
                ' one checkout, which is the defect it exists to catch'
            ),
        ),
        Gate(
            name='ze-platform-vet-darwin',
            argv=('go', 'vet', *PLATFORM_PACKAGES),
            why=(
                'the iface and host trees still compile under GOOS=darwin. Nothing in the'
                ' default host-GOOS build exercises default_other.go, backend_other.go or'
                ' host/platform_other.go, so a stub that stops compiling rots silently'
            ),
        ),
        Gate(
            name='ze-platform-vet-freebsd',
            argv=('go', 'vet', *PLATFORM_PACKAGES),
            why=(
                'the same trees under GOOS=freebsd. An int64-versus-uint64 syscall.Rlimit'
                ' drift is the shape of break this catches'
            ),
        ),
        Gate(
            name='ze-staticcheck-feature-matrix-check',
            argv=(*_go('scripts/checks/staticcheck_feature_matrix.go'), *_words('ARGS')),
            why=(
                'type-check the working tree once per feature-tag configuration.'
                ' ZE_VERIFY_SCOPE_TAGS scopes the rows to the ones a change set can move,'
                ' and unset judges every row the manifest implies. ARGS reaches the'
                " checker's own flags: --print-matrix, --deadline=D"
            ),
        ),
        Gate(
            name='ze-repository-tracked-build-selftest',
            argv=_go('scripts/checks/tracked_build.go', '--selftest'),
            why=(
                'the two vacuity guards still fire. `go build ./...` exits 0 over a pattern'
                ' that matched nothing buildable, so a flavor compiling zero packages would'
                ' otherwise report success'
            ),
        ),
        Gate(
            name='ze-repository-tracked-build-check',
            argv=_tracked_build_argv(),
            why=(
                'compile the tree GIT HOLDS, which is the one population no other check'
                ' compiles. Every other gate builds the working tree, so a consumer'
                ' committed without its producer is green for its author and broken for'
                ' anybody who builds the commit: four commits broke `make ze-build` at HEAD'
                ' that way on 2026-08-04. REV=<commit-ish> judges another commit,'
                ' ARGS=--keep leaves the extracted tree'
            ),
        ),
        Gate(
            name='ze-test-health-update',
            argv=_py('testing_health.py', '--write'),
            why=(
                'regenerate docs/features/test-health.md, its structured sibling'
                ' test/health/latest.json, and the ratchet baseline'
            ),
            writes=True,
        ),
        Gate(
            name='ze-test-health-record',
            argv=_py('testing_health.py', '--record'),
            why=(
                'append ONE KPI row to test/health/history.ndjson, after a mutation or'
                ' verify run. The page renders its trends from the committed history and'
                ' never from live output'
            ),
            writes=True,
        ),
    ),
)


def _environment(gate: Gate) -> dict[str, str]:
    """The environment one gate runs under.

    Only the platform vets need more than the toolchain pins, and what they
    need is a GOOS the build machine is not. The two share an argv, so the
    cross-target is the whole difference between them and it has to come from
    here.
    """
    goos = PLATFORMS.get(gate.name)
    if goos:
        return toolchain().environment(goos=goos)
    return gateapp.default_environment(gate)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select, and refuse a bare `--write`.

    The two generators here do different things to different files, and one of
    them APPENDS: ze-test-health-record adds a KPI sample to the committed
    history, which is a row a caller has to mean rather than a file that can be
    rewritten twice for the same answer. No Make target ever ran both, so there
    is nothing for a bare `--write` to be faithful to.
    """
    if opts.write and not opts.names and not opts.listing:
        echo('repository has no aggregate write: name the generator you want.')
        echo(f'  {", ".join(gate.name for gate in GATES.writers())}')
        return 2
    return gateapp.action(opts, GATES, env=_environment)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
