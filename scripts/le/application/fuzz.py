"""Go fuzzing: every `func Fuzz` under internal/, discovered at run time.

    ./le fuzz                        every target, 10s each
    ./le fuzz --list                 what would run, and where
    ./le fuzz --name FuzzParseOpen   one target, matched by name
    ./le fuzz --name FuzzX --package ./internal/... --time 30s

Ported from mk/test-fuzz.mk and mk/test-fuzz-targets.mk, and it deletes the
second of those rather than moving it.

**A committed generated file existed here only because Make cannot look.** The
enumeration was hand-maintained until it went stale once too often, so
`scripts/dev/fuzz-targets.py` was written to walk `internal/` for `func Fuzz`
and emit one recipe line per target into `mk/test-fuzz-targets.mk`. That file
was then committed, checked for staleness by a third target, and listed in the
regen-check's git-diff guard. Three mechanisms, all of them serving one fact
that a program can simply read from the tree at the moment it needs it.

`discover` below is the same walk. Because it runs when the fuzzers run, there
is no artifact to commit, nothing to go stale, and no check needed to notice
that it has. Adding a fuzzer is adding a `func Fuzz`, with no `make generate`
step between writing it and it being run.

Two constraints on the emitted command are Go fuzz requirements rather than
preferences, and both are why the old generator existed at all:

  The package path is an exact single directory, never `./.../...`. A tree
  with sibling packages (isis/{packet,yang}) makes a wildcard fail with
  "matches more than one package".

  The name is anchored, `-fuzz=^<Name>$`. Without it a target whose name is a
  prefix of another (FuzzParseVPN against FuzzParseVPNAddPath) fails with
  "matches more than one fuzz target".

The admission wrapper stays in Make. `scripts/dev/ze-run.sh` re-enters make to
take a job slot on a shared machine, and dropping it would change what this
target does to everyone else on the box.
"""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Sequence
from dataclasses import dataclass
from pathlib import Path

from le.console import echo
from le.devtools.toolchain import toolchain
from le.paths import REPO_ROOT
from le.process import stream

__all__ = ['Options', 'Target', 'action', 'add_arguments', 'discover', 'main', 'options']

# The budget kept from the historic hand-list: a short stage suitable for
# scheduled CI, with a hard per-target timeout above it.
FUZZTIME = '10s'
TIMEOUT = '60s'

# Directories that never hold fuzz targets and must not be walked.
SKIP_DIRS = frozenset({'vendor', 'tmp', 'testdata', 'node_modules', '.git'})

# Go's rule: a fuzz target is `func Fuzz` or `func FuzzXxx` where Xxx starts
# with an UPPER-case letter. `func Fuzzy` is an ordinary function, and the
# deleted generator's pattern (`Fuzz[A-Za-z0-9_]+`) took it, which would have
# emitted `-fuzz=^Fuzzy$` for a package holding no such target. No such
# function exists in the tree today, so this corrects a latent fault rather
# than a live one -- but the port is the moment to stop carrying it.
FUNC_FUZZ = re.compile(r'^func (Fuzz(?:[A-Z][A-Za-z0-9_]*)?)\s*\(', re.MULTILINE)


@dataclass(frozen=True)
class Target:
    """One fuzz entry point, and the exact package directory holding it."""

    name: str
    package: str

    def command(self, *, fuzztime: str = FUZZTIME, timeout: str = TIMEOUT) -> list[str]:
        """The `go test` invocation for this target, anchored and single-package."""
        return toolchain().go_test(
            f'-fuzz=^{self.name}$',
            f'-fuzztime={fuzztime}',
            f'-timeout={timeout}',
            self.package,
        )


def discover(root: Path = REPO_ROOT) -> list[Target]:
    """Every `func Fuzz` under internal/, sorted by package then name.

    Sorted so a run is reproducible and a package's targets stay together,
    which is what makes an interrupted run readable.
    """
    base = root / 'internal'
    if not base.is_dir():
        return []

    found: set[Target] = set()
    for path in base.rglob('*_test.go'):
        if any(part in SKIP_DIRS for part in path.relative_to(root).parts):
            continue
        text = path.read_text(encoding='utf-8', errors='replace')
        package = './' + path.parent.relative_to(root).as_posix()
        for name in FUNC_FUZZ.findall(text):
            found.add(Target(name=name, package=package))
    return sorted(found, key=lambda t: (t.package, t.name))


@dataclass(frozen=True)
class Options:
    """Everything `action` needs, and nothing about how it was asked for."""

    name: str = ''
    package: str = ''
    fuzztime: str = ''
    timeout: str = TIMEOUT
    listing: bool = False


def add_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument('--list', dest='listing', action='store_true', help='print what would run')
    parser.add_argument('--name', default='', help='run one target, by its `func Fuzz` name')
    parser.add_argument(
        '--package',
        default='',
        help='the package holding it; discovered from the name when omitted',
    )
    parser.add_argument(
        '--time',
        dest='fuzztime',
        default='',
        help=f'fuzz duration per target (default {FUZZTIME}, or 30s with --name)',
    )
    parser.add_argument('--timeout', default=TIMEOUT, help=f'hard timeout (default {TIMEOUT})')


def options(namespace: argparse.Namespace) -> Options:
    return Options(
        name=str(namespace.name),
        package=str(namespace.package),
        fuzztime=str(namespace.fuzztime),
        timeout=str(namespace.timeout),
        listing=bool(namespace.listing),
    )


def action(opts: Options) -> int:
    """Run the selected fuzz targets. Returns the process exit code."""
    if opts.name or opts.package:
        return _run_one(opts)

    targets = discover()
    if not targets:
        echo('no `func Fuzz` found under internal/')
        return 1

    fuzztime = opts.fuzztime or FUZZTIME
    if opts.listing:
        for target in targets:
            echo(f'  {target.name:<40} {target.package}')
        echo(f'{len(targets)} fuzz target(s), {fuzztime} each')
        return 0

    echo(f'Running {len(targets)} fuzz target(s), {fuzztime} each...')
    env = toolchain().environment(procs=True)
    for target in targets:
        echo(f'==> {target.name} ({target.package})')
        code = stream(
            target.command(fuzztime=fuzztime, timeout=opts.timeout), cwd=REPO_ROOT, env=env
        )
        if code != 0:
            # STOP, as the Make recipe did. Each `$(GO_TEST)` line was its own
            # recipe line, so the first non-zero aborted the target and the
            # remaining fuzzers never ran. Continuing would spend another
            # ~13 minutes fuzzing after a crash is already in hand, and bury
            # the failure that matters under everything that followed it.
            echo()
            echo(f'Failed: {target.name} in {target.package}')
            return code
    echo()
    echo(f'{len(targets)} fuzz target(s) passed.')
    return 0


def _run_one(opts: Options) -> int:
    """Run exactly what the caller named, without consulting discovery.

    This is `ze-fuzz-test-one`, and its contract is the recipe's:
    `$(GO_TEST) -fuzz=$(FUZZ) -fuzztime=$(TIME) $(PKG)`. Both arguments went
    to Go untouched, so `FUZZ` was a Go REGEXP and `PKG` could be a `/...`
    wildcard -- the documented usage line is
    `PKG=./internal/component/bgp/wireu/...`.

    An earlier version of this filtered discovery by exact equality on both,
    so the documented invocation exited 2 and the regexp form stopped working.
    Discovery exists to enumerate ALL targets; it has no business narrowing
    one the caller has already named.
    """
    name = opts.name or 'Fuzz'
    package = opts.package or './internal/...'
    fuzztime = opts.fuzztime or '30s'

    argv = toolchain().go_test(
        f'-fuzz={name}',
        f'-fuzztime={fuzztime}',
        f'-timeout={opts.timeout}',
        package,
    )
    if opts.listing:
        echo('  ' + ' '.join(argv))
        return 0
    echo(f'==> {name} ({package}), {fuzztime}')
    return stream(argv, cwd=REPO_ROOT, env=toolchain().environment(procs=True))


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog='le fuzz', description=__doc__)
    add_arguments(parser)
    return action(options(parser.parse_args(argv)))


if __name__ == '__main__':
    sys.exit(main())
