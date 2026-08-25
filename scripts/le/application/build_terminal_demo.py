"""Website terminal demonstrations: the binaries they record, and the recordings.

Ported from mk/build-terminal-demo.mk. Six targets moved; that file's header
lists what stayed and why.

    ./le build-terminal-demo                            every check
    ./le build-terminal-demo --list                     what each one is for
    ./le build-terminal-demo --write                    the binaries, then a re-render
    ./le build-terminal-demo ze-terminal-demo-check-all one of them

Two kinds of gate live here. The first pair cross-builds the two binaries a
demo drives, and the rest call demos/terminal/render.py over the whole
manifest.

The binaries are TARGET binaries: the renders happen inside a pinned container
on Docker's native Linux architecture, so a recording is reproducible and needs
no emulation on Apple Silicon. GOOS is therefore linux whatever the host is,
and GOARCH follows the host so the container runs them natively. `ze` carries
the shipped feature set plus ze_distro; `ze-test` carries ze_test alone and no
version, because a demo never shows the test runner's version.

The container has no external network and takes only the capabilities the Linux
network-namespace traceroute lab needs. The host needs docker and python3.
ffmpeg is read by the ONE browser demo, whose video render.py rescales and
whose poster it resizes; a terminal demo records an asciicast and needs none of
it. It is the operator's to install: the recorder opens its own PTY since
2026-08-24, so the install target that used to place it beside VHS and ttyd was
deleted with them.

Three values the Makefile spelled as overridable variables are read from the
environment here, with the same defaults. A `make <target> VAR=value` reaches
this program, because GNU make puts a command-line variable into the recipe
environment.
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
from collections.abc import Sequence
from pathlib import Path

from le import gateapp
from le.devtools.gate import Gate, GateSet
from le.devtools.toolchain import toolchain
from le.paths import REPO_ROOT

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']

# The renderer, and the manifest it reads to know what a demo is.
RENDER = 'demos/terminal/render.py'

# Where the binaries a demo drives are staged for the container to mount.
BIN_DIR = f'{REPO_ROOT}/tmp/terminal-demos/bin'


def _release() -> str:
    """The release identity recorded in artifact metadata.

    `TERMINAL_DEMO_RELEASE` when the caller set one, otherwise this run's
    version. Both come from the clock, so the recording and the binary it
    recorded agree by construction.
    """
    return os.environ.get('TERMINAL_DEMO_RELEASE') or toolchain().version


def _output() -> str:
    """Where rendered assets land: the published website checkout beside this one."""
    return os.environ.get('TERMINAL_DEMO_OUTPUT') or f'{REPO_ROOT}/../gh-pages/assets/demos'


def _goarch() -> str:
    """The architecture the renderer container runs natively.

    The host's, asked of the toolchain rather than assumed, so the container
    executes the binaries instead of emulating them.
    """
    override = os.environ.get('TERMINAL_DEMO_GOARCH')
    if override:
        return override
    found = subprocess.run(['go', 'env', 'GOARCH'], capture_output=True, text=True, check=False)
    return found.stdout.strip()


def _demo_tags() -> str:
    """`ze_core ze_distro` plus every feature gate, which is what a demo shows.

    ze_distro rather than ze_appliance: a demo shows the distribution build,
    the one an operator installs on a machine they already have.
    """
    tc = toolchain()
    return ' '.join(('ze_core', 'ze_distro', *tc.features, *tc.extra_tags))


GATES = GateSet(
    area='build-terminal-demo',
    gates=(
        Gate(
            name='ze-terminal-demo-check-all',
            argv=('python3', RENDER, '--all', '--check'),
            why='every demo the manifest declares has its published artifacts',
        ),
        Gate(
            name='ze-terminal-demo-validation-check-all',
            argv=('python3', RENDER, '--all', '--validate'),
            why="each scenario's output validators pass, so a demo shows the product working",
        ),
        Gate(
            name='ze-terminal-demo-release-check-all',
            argv=('python3', RENDER, '--all', '--release', _release(), '--check'),
            why='the published artifacts carry this release identity, which is what a tag ships',
        ),
        Gate(
            name='ze-terminal-demo-binaries-build-ze',
            argv=(
                'go',
                'build',
                '-tags',
                _demo_tags(),
                '-ldflags',
                toolchain().ldflags,
                '-o',
                f'{BIN_DIR}/ze',
                './cmd/ze',
            ),
            why='the ze a demo drives, cross-built for the renderer container',
            writes=True,
        ),
        Gate(
            name='ze-terminal-demo-binaries-build-ze-test',
            argv=('go', 'build', '-tags', 'ze_test', '-o', f'{BIN_DIR}/ze-test', './cmd/ze'),
            why='the ze-test a demo drives, which carries ze_test alone and no version',
            writes=True,
        ),
        Gate(
            name='ze-terminal-demo-render-all',
            argv=('python3', RENDER, '--all', '--release', _release()),
            why='re-record every website demo from its checked-in tape',
            writes=True,
        ),
    ),
)


def _environment(gate: Gate) -> dict[str, str]:
    """The environment one gate runs under, read off the command it runs.

    A build cross-compiles for the container. A render reads
    ZE_TERMINAL_DEMO_OUTPUT to decide which asset tree it is working on;
    without it a check would read the renderer's own default and report on
    assets nobody publishes.
    """
    if gate.argv[0] == 'go':
        return toolchain().environment(goos='linux', goarch=_goarch())
    env = dict(os.environ)
    env['ZE_TERMINAL_DEMO_OUTPUT'] = _output()
    return env


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select, each gate under its own Go environment.

    `_environment` is what this area adds: the shared helper's default
    carries the toolchain pins, and these gates need more than that
    (CGO_ENABLED for a race build, GOMAXPROCS for a test run).
    """
    if not opts.listing:
        # `mkdir -p $(TERMINAL_DEMO_BIN_DIR)` was the first line of the
        # pre-port binaries-build recipe, and it is not decoration: BIN_DIR
        # lives under gitignored tmp/, so it is absent on a fresh checkout and
        # after every scratch wipe, and `go build -o` into a missing directory
        # fails. A Gate is a command, so it has nowhere to carry this.
        Path(BIN_DIR).mkdir(parents=True, exist_ok=True)
    return gateapp.action(opts, GATES, env=_environment)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
