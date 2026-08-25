"""The installer initrd's PID 1, cross-built for both appliance architectures.

Ported from mk/build-appliance.mk. ONE target moved; that file's header lists
the six that stayed and why.

    ./le build-appliance --list                       what each gate builds
    ./le build-appliance --write                      both architectures
    ./le build-appliance ze-installer-build-arm64     one of them

cmd/ze-installer is a TARGET binary, never a host one. It is packed into the
installer initrd as /init and it is the first process the appliance runs, so it
is cross-compiled `GOOS=linux GOARCH=<arch> CGO_ENABLED=0` and it is built for
both architectures every time. Applying a target GOARCH to a HOST tool is the
mistake this distinction exists to prevent: a target-arch host binary cannot
exec on the build machine, and says so with "exec format error".

The two gates land at bin/ze-installer-<arch>, which is the standalone
cross-build path the naming convention names. That is the repository's bin/,
not a session directory: these are release artifacts rather than something a
session runs.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.console import echo
from le.devtools.gate import Gate, GateSet, run_gate
from le.devtools.toolchain import toolchain
from le.paths import REPO_ROOT

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']

# Where a standalone installer cross-build lands. Not a session directory: an
# artifact somebody flashes is not a per-session binary.
OUTPUT_DIR = 'bin'

# The one tag the installer builds under. cmd/ze-installer is guarded by
# //go:build ze_installer from end to end, and a build without it is empty.
INSTALLER_TAG = 'ze_installer'

# The architectures an appliance runs on. Both are built every time, because a
# release that ships one of them is a release that boots on one of them.
ARCHITECTURES: tuple[str, ...] = ('amd64', 'arm64')


def _build(arch: str) -> tuple[str, ...]:
    """The cross build for one architecture, argument for argument."""
    return (
        'go',
        'build',
        '-tags',
        INSTALLER_TAG,
        '-ldflags',
        toolchain().ldflags,
        '-o',
        f'{OUTPUT_DIR}/ze-installer-{arch}',
        './cmd/ze-installer',
    )


GATES = GateSet(
    area='build-appliance',
    gates=tuple(
        Gate(
            name=f'ze-installer-build-{arch}',
            argv=_build(arch),
            why=f'the installer initrd PID 1 for {arch}, at {OUTPUT_DIR}/ze-installer-{arch}',
            writes=True,
        )
        for arch in ARCHITECTURES
    ),
)


def _environment(gate: Gate) -> dict[str, str]:
    """The cross-compilation environment one gate builds under.

    The architecture is read off the output path the gate already carries, so
    the two cannot disagree about which binary is being built.
    """
    arch = gate.name.rsplit('-', 1)[1]
    return toolchain().environment(goos='linux', goarch=arch)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select, each under its own GOARCH.

    The shared helper cannot run these: `gateapp.action` calls `run_all` with
    one environment, and a cross build is exactly a per-gate environment. It
    also never creates the output directory, and `go build -o` does not either.
    """
    if opts.listing:
        echo(f'{GATES.area}:')
        GATES.render_list()
        return 0

    chosen = _chosen(opts)
    if isinstance(chosen, int):
        return chosen

    if opts.as_json:
        echo(f'{GATES.area} has no machine-readable report')
        return 2

    if chosen:
        (REPO_ROOT / OUTPUT_DIR).mkdir(parents=True, exist_ok=True)

    failed = [gate.name for gate in chosen if run_gate(gate, env=_environment(gate)) != 0]
    echo()
    if failed:
        echo(f'Failed: {", ".join(failed)}')
        return 1
    echo(f'{GATES.area}: {len(chosen)} gate(s) passed.')
    return 0


def _chosen(opts: gateapp.Options) -> tuple[Gate, ...] | int:
    """The gates the options select, or the exit code to return instead.

    Every gate here writes, so a bare `le build-appliance` selects none of
    them. Building a release artifact is not something a check sweep does by
    walking into it.
    """
    if not opts.names:
        return GATES.writers() if opts.write else GATES.checks()
    selected: list[Gate] = []
    for name in opts.names:
        gate = GATES.find(name)
        if gate is None:
            echo(f'no such gate in {GATES.area}: {name}')
            echo(f'try one of: {", ".join(GATES.names())}')
            return 2
        selected.append(gate)
    return tuple(selected)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
