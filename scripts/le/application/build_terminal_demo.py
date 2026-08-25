"""Website terminal demonstrations: verify what is published, or render it again.

Ported from mk/build-terminal-demo.mk. The four RENDER-DRIVER targets moved;
that file's header lists what stayed and why.

    ./le build-terminal-demo                            every check
    ./le build-terminal-demo --list                     what each one is for
    ./le build-terminal-demo --write                    re-render every demo
    ./le build-terminal-demo ze-terminal-demo-check-all one of them

Every gate here is one call into demos/terminal/render.py over the whole
manifest. The renders themselves happen inside a pinned container built on
Docker's native Linux architecture, so a render is reproducible and needs no
emulation on Apple Silicon. That container has no external network and takes
only the capabilities the Linux network-namespace traceroute lab needs.

The host needs docker and python3. ffmpeg is read by the ONE browser demo,
whose video render.py rescales and whose poster it resizes; a terminal demo
records an asciicast and needs none of it. It is the operator's to install:
the recorder opens its own PTY since 2026-08-24, so the install target that
used to place it beside VHS and ttyd was deleted with them.

Two values the Makefile spelled as overridable variables are read from the
environment here, with the same defaults. A `make <target> VAR=value` reaches
this program, because GNU make puts a command-line variable into the recipe
environment.
"""

from __future__ import annotations

import argparse
import os
import sys
from collections.abc import Sequence
from datetime import datetime

from le import gateapp
from le.console import echo
from le.devtools.gate import Gate, GateSet, run_gate
from le.paths import REPO_ROOT

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options', 'render_env']

# The renderer, and the manifest it reads to know what a demo is.
RENDER = 'demos/terminal/render.py'


def _release() -> str:
    """The release identity recorded in artifact metadata.

    `TERMINAL_DEMO_RELEASE` when the caller set one, otherwise the same
    YY.MM.DD the Makefile derives for ZE_VERSION. Derived from the clock rather
    than copied from anywhere, so the two agree by construction.
    """
    return os.environ.get('TERMINAL_DEMO_RELEASE') or datetime.now().strftime('%y.%m.%d')


def _output() -> str:
    """Where rendered assets land: the published website checkout beside this one."""
    return os.environ.get('TERMINAL_DEMO_OUTPUT') or f'{REPO_ROOT}/../gh-pages/assets/demos'


def render_env() -> dict[str, str]:
    """The environment render.py runs under.

    One variable, and it is the only thing separating a check of the published
    assets from a check of somebody's scratch copy.
    """
    env = dict(os.environ)
    env['ZE_TERMINAL_DEMO_OUTPUT'] = _output()
    return env


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
            name='ze-terminal-demo-render-all',
            argv=('python3', RENDER, '--all', '--release', _release()),
            why='re-record every website demo from its checked-in tape',
            writes=True,
        ),
    ),
)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select, with the renderer's output directory set.

    The shared helper cannot run these: `gateapp.action` calls `run_all` with
    no environment, and render.py reads ZE_TERMINAL_DEMO_OUTPUT to decide which
    tree it is checking. Without it a check would read the renderer's own
    default and report on assets nobody publishes.
    """
    if opts.listing:
        echo(f'{GATES.area}:')
        GATES.render_list()
        return 0

    chosen: tuple[Gate, ...]
    if opts.names:
        selected: list[Gate] = []
        for name in opts.names:
            gate = GATES.find(name)
            if gate is None:
                echo(f'no such gate in {GATES.area}: {name}')
                echo(f'try one of: {", ".join(GATES.names())}')
                return 2
            selected.append(gate)
        chosen = tuple(selected)
    else:
        chosen = GATES.writers() if opts.write else GATES.checks()

    if opts.as_json:
        echo(f'{GATES.area} has no machine-readable report')
        return 2

    env = render_env()
    failed = [gate.name for gate in chosen if run_gate(gate, env=env) != 0]
    echo()
    if failed:
        echo(f'Failed: {", ".join(failed)}')
        return 1
    echo(f'{GATES.area}: {len(chosen)} gate(s) passed.')
    return 0


def main(argv: Sequence[str] | None = None) -> int:
    """Standalone entry: parse, then call THIS module's action.

    Not `gateapp.main`, which would call the shared action and drop the
    environment above. The two routes into a subprogram are required to agree
    (le/registry.py), so the standalone route has to reach the same code the
    dispatcher reaches.
    """
    parser = argparse.ArgumentParser(prog=f'le {GATES.area}', description=__doc__)
    add_arguments(parser)
    return action(options(parser.parse_args(argv)))


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
