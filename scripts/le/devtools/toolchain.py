"""The Go toolchain environment every build and test command runs under.

The Makefile computes these at the top of every run and exports them. They are
derived here from the same sources, never copied: a literal here and the value
in `go.mod` or `feature-gates.txt` would be two records of one fact, and the
one nothing compares is the one that drifts.

Four of them are load-bearing in ways their names do not say:

  GOCACHE is inside the checkout, not in TMPDIR. A cache under TMPDIR breaks
  the Unix-socket tests, whose socket paths would exceed the length the kernel
  allows. It sits on the durable side (`cache/`, symlinked out of tree) rather
  than in `tmp/`, so it survives a scratch wipe.

  CGO_ENABLED is 0 by default. The race detector needs 1, and only the race
  commands set it.

  GOTOOLCHAIN comes from the `toolchain` line of go.mod. golangci-lint is a
  separate binary linked against one version of go/types, and it decodes the
  export data the ambient toolchain writes. An ambient Go newer than the one
  the linter was built with makes every package fail to typecheck with "export
  data version N is greater than maximum supported version M", while the linter
  prints "0 issues" and exits non-zero. Measured 2026-08-22 with ambient Go
  1.27.0 against golangci-lint built with 1.26.6. A warm GOCACHE hides it: the
  entries were written by whatever toolchain ran first, so the break appears
  only on a cold cache, which is when nobody is looking for a toolchain fault.

  GOMAXPROCS is a QUARTER of the cores. No Go process or test run gets more.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from functools import lru_cache
from pathlib import Path

from le.paths import REPO_ROOT

__all__ = ['Toolchain', 'toolchain']

# Where the feature gates are declared. One line per tag, shared with the
# generator, dep_audit.py and the test runner. A gate is added THERE.
FEATURE_GATES = 'feature-gates.txt'

# The default per-test-binary timeout, overridable the way Make allowed.
DEFAULT_TEST_TIMEOUT = '20m'


def _feature_tags(root: Path) -> tuple[str, ...]:
    """Every `ze_*` tag declared in feature-gates.txt, sorted and deduplicated.

    Derived rather than listed, so adding a gate is one line in that file.
    """
    path = root / FEATURE_GATES
    try:
        lines = path.read_text().splitlines()
    except OSError:
        return ()
    found = {
        line.split()[0] for line in lines if line.split() and line.split()[0].startswith('ze_')
    }
    return tuple(sorted(found))


def _go_toolchain(root: Path) -> str:
    """The `toolchain` line of go.mod, or the empty string when it has none.

    Empty leaves the ambient toolchain untouched, which is the behaviour before
    the pin existed.
    """
    try:
        lines = (root / 'go.mod').read_text().splitlines()
    except OSError:
        return ''
    for line in lines:
        parts = line.split()
        if len(parts) >= 2 and parts[0] == 'toolchain':
            return parts[1]
    return ''


def _test_procs() -> int:
    """A quarter of the cores, and never fewer than one."""
    return max(1, (os.cpu_count() or 4) // 4)


@dataclass(frozen=True)
class Toolchain:
    """The environment and the command prefixes derived from this checkout."""

    root: Path
    features: tuple[str, ...]
    go_toolchain: str
    procs: int
    timeout: str
    extra_tags: tuple[str, ...] = field(default=())

    @property
    def test_tags(self) -> str:
        """The tag set a normal build and test carries: core, every feature, extras."""
        return ' '.join(('ze_core', *self.features, *self.extra_tags))

    @property
    def core_tags(self) -> str:
        """The bare tag set, for the compile-out checks.

        A reduced set compiles modules out and shrinks the surface a gate can
        see, which is the point for those checks and a defect everywhere else.
        """
        return ' '.join(('ze_core', *self.extra_tags))

    def environment(self, *, cgo: bool = False, procs: bool = False) -> dict[str, str]:
        """The environment a Go command runs under.

        `cgo` is for the race detector, which cannot run without it. `procs`
        adds the GOMAXPROCS cap, which belongs on a test run rather than on a
        build.
        """
        env = dict(os.environ)
        env['GOCACHE'] = str(self.root / 'cache' / 'go-cache')
        env['GOLANGCI_LINT_CACHE'] = str(self.root / 'tmp' / 'golangci-lint-cache')
        env['CGO_ENABLED'] = '1' if cgo else '0'
        if self.go_toolchain:
            env['GOTOOLCHAIN'] = self.go_toolchain
        if procs:
            env['GOMAXPROCS'] = str(self.procs)
        return env

    def go_run(self, script: str, *args: str) -> list[str]:
        """`go run` over one script, carrying the full feature tag set.

        The full set matters for the gates that read the command surface: a
        reduced set compiles modules out, and the gate then reports on a
        smaller product than the one that ships.
        """
        return ['go', 'run', '-tags', self.test_tags, script, *args]

    def go_test(self, *args: str, core: bool = False, race: bool = False) -> list[str]:
        """`go test` with this checkout's tags, timeout and flags."""
        tags = self.core_tags if core else self.test_tags
        argv = ['go', 'test', '-timeout', self.timeout, '-tags', tags]
        if race:
            argv.append('-race')
        argv.extend(args)
        return argv


@lru_cache(maxsize=1)
def toolchain(root: Path = REPO_ROOT) -> Toolchain:
    """This checkout's toolchain, computed once per process.

    Cached because every value is read from a file that does not change during
    a run, and a subprogram that runs forty gates would otherwise re-read
    go.mod and feature-gates.txt forty times.
    """
    extra = tuple(tag for tag in os.environ.get('ZE_TAGS', '').split() if tag)
    return Toolchain(
        root=root,
        features=_feature_tags(root),
        go_toolchain=_go_toolchain(root),
        procs=_test_procs(),
        timeout=os.environ.get('GO_TEST_TIMEOUT', DEFAULT_TEST_TIMEOUT),
        extra_tags=extra,
    )
