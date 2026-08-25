"""Lint and type-check the Python half of the tree.

    PYTHONPATH=scripts python3 -m le.application.lint
    ./le lint --fix

Two scopes, because the tree is two populations and measuring them says so.
The strict rule set reports 59,207 findings against the code written before any
of this existed, and 53,625 of those are the quote style alone. A gate nobody
can pass is not a gate.

    strict   scripts/le and ./le. Every ruff rule, enforced formatting, and
             mypy --strict. Clean from the first commit, so there is nothing to
             ratchet down from.
    legacy   everything else. Real defect shapes only -- undefined names,
             mutable defaults, loop-variable capture -- and no style at all.
             116 findings today, held to a ceiling that must fall rather than
             to zero it cannot reach.

Neither rule set is written here. Ruff reads pyproject.toml for the legacy tree
and scripts/le/ruff.toml for the strict scope, choosing per file by which
config is nearest, so one `ruff check` applies both and an editor sees exactly
what this subprogram sees.
"""

from __future__ import annotations

import argparse
import sys
import tomllib
from collections.abc import Sequence
from dataclasses import dataclass

from le.console import echo
from le.paths import REPO_ROOT
from le.process import Command, run, which

__all__ = ['Options', 'action', 'add_arguments', 'legacy_ceiling', 'main', 'options']

# The strictly-checked paths, relative to the checkout. `le` is the entry-point
# shim at the root; it is one file and it is held to the same standard.
STRICT_SCOPE: tuple[str, ...] = ('scripts/le', 'le')

# Where the ratchet's ceiling is recorded. One number, beside the rules it
# counts against, so a commit that lowers it shows both in one diff.
PYPROJECT = 'pyproject.toml'


@dataclass(frozen=True)
class Options:
    """Everything `action` needs, and nothing about how it was asked for."""

    fix: bool = False
    types_only: bool = False
    lint_only: bool = False
    strict_only: bool = False


def add_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument(
        '--fix',
        action='store_true',
        help='apply the fixes ruff can make, and format, instead of only reporting',
    )
    parser.add_argument(
        '--strict-only',
        action='store_true',
        help='check scripts/le alone; skip the legacy-tree ratchet',
    )
    group = parser.add_mutually_exclusive_group()
    group.add_argument('--types-only', action='store_true', help='run the type checker alone')
    group.add_argument('--lint-only', action='store_true', help='run the linter alone')


def options(namespace: argparse.Namespace) -> Options:
    """Turn the parsed namespace into the typed value `action` takes."""
    return Options(
        fix=bool(namespace.fix),
        types_only=bool(namespace.types_only),
        lint_only=bool(namespace.lint_only),
        strict_only=bool(namespace.strict_only),
    )


def legacy_ceiling() -> int:
    """The most legacy findings this repository currently tolerates.

    Read from pyproject.toml rather than written here: the number and the rules
    it counts against are one fact, and the copy nothing compares is the one
    that goes stale.
    """
    with (REPO_ROOT / PYPROJECT).open('rb') as handle:
        config = tomllib.load(handle)
    tool = config.get('tool', {})
    value = tool.get('le', {}).get('lint', {}).get('legacy-max')
    if not isinstance(value, int):
        raise ValueError(f'{PYPROJECT} has no integer [tool.le.lint] legacy-max')
    return value


def action(opts: Options) -> int:
    """Run the checks. Returns nonzero if any of them found something.

    Every stage runs even after one fails. A run that stops at the first red
    reports one thing per invocation, and the point of a lint pass is to hand
    back the whole list.
    """
    failures: list[str] = []

    if not opts.types_only:
        failures.extend(_ruff_strict(opts.fix))
    if not opts.lint_only:
        failures.extend(_mypy())
    if not opts.strict_only and not opts.types_only:
        failures.extend(_ruff_legacy())

    echo()
    if failures:
        echo(f'Failed: {", ".join(failures)}')
        return 1
    echo('Python lint and types clean.')
    return 0


def _ruff_strict(fix: bool) -> list[str]:
    """Lint and format-check the strict scope. Returns the stages that failed."""
    if which('ruff') is None:
        echo('  SKIP ruff: not installed; run `./le setup`')
        return ['ruff (not installed)']

    failed: list[str] = []

    check: Command = ['ruff', 'check', *STRICT_SCOPE]
    if fix:
        check.append('--fix')
    if not _stage(f'ruff check {" ".join(STRICT_SCOPE)}', check):
        failed.append('ruff check')

    fmt: Command = ['ruff', 'format', *STRICT_SCOPE]
    if not fix:
        fmt.append('--check')
    if not _stage(f'ruff format {" ".join(STRICT_SCOPE)}', fmt):
        failed.append('ruff format')

    return failed


def _ruff_legacy() -> list[str]:
    """Count findings outside the strict scope and hold them to the ceiling.

    A ratchet rather than a gate, because the count is not zero and pretending
    otherwise would mean either excluding the tree (no coverage) or failing
    every run (no signal). It fails on an INCREASE, and it says so when the
    count has fallen, because a ceiling nobody lowers is a ceiling that stops
    meaning anything.
    """
    if which('ruff') is None:
        return []

    ceiling = legacy_ceiling()
    echo(f'==> ruff check (legacy tree, ceiling {ceiling})')

    # `--statistics` for the count, so the report is a table of rule totals
    # rather than 116 diagnostics nobody reads on a green run.
    result = run(['ruff', 'check', '--statistics', '--exclude', 'scripts/le'], cwd=REPO_ROOT)
    found = _count_findings(result.out)

    if found > ceiling:
        echo(result.out.strip())
        echo(f'  {found} findings, {found - ceiling} over the ceiling of {ceiling}')
        echo(f'  Fix them, or raise [tool.le.lint] legacy-max in {PYPROJECT} and say why')
        return ['ruff check (legacy)']

    if found < ceiling:
        echo(f'  {found} findings, {ceiling - found} under the ceiling')
        echo(f'  Lower [tool.le.lint] legacy-max to {found} in {PYPROJECT}')
        return ['ruff check (legacy ceiling is stale)']

    echo(f'  {found} findings, at the ceiling')
    return []


def _count_findings(statistics: str) -> int:
    """Total the counts in `ruff check --statistics` output.

    Each line begins with a count and a rule code. Summing them is what gives a
    single number to ratchet on; the per-rule breakdown is what a reader needs
    when it goes red, which is why both come from one invocation.
    """
    total = 0
    for line in statistics.splitlines():
        head = line.split(maxsplit=1)
        if head and head[0].isdigit():
            total += int(head[0])
    return total


def _mypy() -> list[str]:
    """Type-check the strict scope. Returns the stages that failed.

    Not installed is a FAILURE, not a skip. mypy is a required tool, and a gate
    that reports green because its checker is absent is exactly the shape this
    repository has been bitten by: it reads as "checked" when it means "not
    checked".
    """
    if which('mypy') is None:
        echo('  SKIP mypy: not installed; run `./le setup`')
        return ['mypy (not installed)']

    # The scope comes from pyproject.toml's `files`, so it is not repeated
    # here.
    if not _stage('mypy --strict', ['mypy']):
        return ['mypy']
    return []


def _stage(label: str, argv: Command) -> bool:
    """Run one checker and show what it said. Returns whether it passed."""
    echo(f'==> {label}')
    result = run(argv, cwd=REPO_ROOT)
    for stream in (result.out, result.err):
        text = stream.strip()
        if text:
            echo(text)
    return result.ok


def main(argv: Sequence[str] | None = None) -> int:
    """Standalone entry. Parses, builds options, and calls `action`."""
    parser = argparse.ArgumentParser(prog='le lint', description=__doc__)
    add_arguments(parser)
    return action(options(parser.parse_args(argv)))


if __name__ == '__main__':
    sys.exit(main())
