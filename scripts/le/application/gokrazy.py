"""The gokrazy appliance build's two one-command steps.

Ported from mk/build-gokrazy.mk. TWO targets moved; that file's header lists
the seven that stayed and why.

    ./le gokrazy                              the check
    ./le gokrazy --list                       what each one is for
    ./le gokrazy --write                      build ze-host
    ./le gokrazy ze-gokrazy-gosum-check       one of them

Almost nothing in that file is a gate. An image build is a shell program: a
credential branch over USER, PASS, ZEFS and CERTNAME, an mkfs, a debugfs write
and a read-back comparison; the kernel path is a cache HIT/MISS branch with a
staged rename and a `$(MAKE) -C gokrazy/kernel`; `ze-gokrazy-run` and
`ze-kernel-clean` are a `case` and a migration. All of that stayed.

**ze-host is a HOST binary and it is built as one.** No GOOS or GOARCH override
reaches it, and the target architecture is passed to `ze appliance kernel` as
`--arch` instead. A target-arch host tool cannot exec on the build machine, and
says so with "exec format error" (CLAUDE.md, "Binary naming convention").

Its tag set is `ze_core ze_setup` and carries no feature gates. The appliance
verb comes from the blank import in cmd/ze/setup_features_setup.go, which is
`//go:build ze_setup`; feature gates select DAEMON features and have nothing to
say about a build driver (scripts/evidence/feature_tags_test.py).
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from le import gateapp
from le.devtools.gate import Gate, GateSet
from le.paths import REPO_ROOT

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']

# The tag set the host build driver carries, and the whole of it. NOT the
# feature set: see the module docstring.
HOST_TAGS = 'ze_core ze_setup'

GATES = GateSet(
    area='gokrazy',
    gates=(
        Gate(
            name='ze-gokrazy-gosum-check',
            argv=('python3', 'scripts/dev/gokrazy_gosum_check.py'),
            why=(
                'the packed gokrazy/ze/builddir/**/go.sum files agree with the root'
                ' module about what a version contains. No other build reads them, so a'
                ' drift there surfaces nowhere else and ships in the image'
            ),
        ),
        Gate(
            name='ze-host-build',
            argv=(
                'go',
                'build',
                '-tags',
                HOST_TAGS,
                '-o',
                str(REPO_ROOT / 'ze-host'),
                './cmd/ze',
            ),
            why=(
                'ze-host, the `ze appliance ...` driver that runs on the BUILD machine.'
                ' It owns the kernel cache key, so every QEMU target declares it as a'
                ' prerequisite before taking the staged-kernel guard'
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
