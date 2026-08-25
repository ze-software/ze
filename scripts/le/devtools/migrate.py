"""Move a directory of Python scripts into `le` as an importable package.

Ze's Python is 466 files in seven directories, and only `scripts/le` is held to
the full rule set plus `mypy --strict`. Bringing a directory under `le` is what
makes its code answer to that standard, and what lets `le` IMPORT a gate rather
than fork an interpreter to reach it (`le/devtools/inproc.py`).

Three things have to happen together, and doing any one alone leaves the tree
broken:

    the files move        git mv, so history follows
    89 files are RENAMED  a hyphen is legal in a filename and not in a module
                          name, so `ensure-links.py` can be executed but never
                          imported. `import ensure-links` is a syntax error
    every reference moves ~2000 of them across Go, Python, shell, Make, YAML
                          and Markdown

This plans all three and applies them in one pass, so the tree is never half
moved. `--dry-run` prints the plan and touches nothing, which is how a
directory is judged before it is moved.

**What this deliberately does NOT decide.** A directory whose Python is wedded
to data is not a package: `test/interop` is 169 scripts against 325 fixtures,
`demos/terminal` 5 against 133 recordings. Severing those to satisfy a layout
would be the wrong trade, and this tool will move them if pointed at them. The
judgement stays with the person running it.
"""

from __future__ import annotations

import re
import subprocess
from collections.abc import Iterable, Sequence
from dataclasses import dataclass, field
from pathlib import Path

from le.paths import REPO_ROOT

__all__ = ['Move', 'Plan', 'module_name', 'plan_move']

# Files that are executed but never imported, and directories that hold no
# code of ours.
SKIP_DIRS = frozenset({'vendor', 'tmp', 'cache', '__pycache__', '.git', 'node_modules'})

# Where a reference can hide. Markdown dominates by count; Go and Make dominate
# by consequence.
SEARCHED = ('*.py', '*.go', '*.sh', '*.mk', '*.md', '*.yml', '*.yaml', '*.json', 'Makefile')


def module_name(filename: str) -> str:
    """The importable name for a script's file name.

    A hyphen is the whole problem: `ensure-links.py` runs and cannot be
    imported, because `import ensure-links` does not parse. Underscores are the
    only mechanical fix, and the rename is why a move costs more than a `git mv`.
    """
    stem = filename[:-3] if filename.endswith('.py') else filename
    return stem.replace('-', '_').replace('.', '_')


@dataclass(frozen=True)
class Move:
    """One file, from where it is to where it will be."""

    source: Path
    target: Path
    renamed: bool

    @property
    def module(self) -> str:
        return self.target.stem


@dataclass
class Plan:
    """Everything one directory's move will do, before any of it happens."""

    package: str
    moves: list[Move] = field(default_factory=list)
    sibling_imports: dict[str, list[str]] = field(default_factory=dict)
    references: dict[str, int] = field(default_factory=dict)

    @property
    def renames(self) -> list[Move]:
        return [m for m in self.moves if m.renamed]

    def render(self) -> str:
        lines = [
            f'package: le.{self.package}',
            f'files:   {len(self.moves)}  ({len(self.renames)} renamed for importability)',
            f'sibling imports to rewrite: {sum(len(v) for v in self.sibling_imports.values())}',
            f'files holding a reference:  {len(self.references)}',
        ]
        if self.renames:
            lines.append('')
            lines.append('renamed:')
            lines.extend(f'  {m.source.name} -> {m.target.name}' for m in self.renames[:12])
            if len(self.renames) > 12:
                lines.append(f'  ... and {len(self.renames) - 12} more')
        return '\n'.join(lines)


def _python_files(directory: Path) -> list[Path]:
    return sorted(
        p for p in directory.rglob('*.py') if not any(part in SKIP_DIRS for part in p.parts)
    )


def _sibling_modules(files: Iterable[Path]) -> set[str]:
    """The importable names a file in this directory could `import` directly.

    Both spellings: the name on disk and the name after renaming, because a
    sibling import today names the file as it is now.
    """
    names: set[str] = set()
    for path in files:
        names.add(path.stem)
        names.add(module_name(path.name))
    return names


def _find_sibling_imports(files: Sequence[Path], siblings: set[str]) -> dict[str, list[str]]:
    """Which files import a sibling, and which sibling.

    A sibling import works today only because Python puts a script's own
    directory on `sys.path` when it RUNS it. As a package module it does not,
    so each one becomes an absolute import of the new package.
    """
    found: dict[str, list[str]] = {}
    pattern = re.compile(r'^\s*(?:import|from)\s+([a-z_][a-z0-9_]*)', re.MULTILINE)
    for path in files:
        try:
            text = path.read_text(encoding='utf-8', errors='replace')
        except OSError:
            continue
        hits = sorted({m for m in pattern.findall(text) if m in siblings})
        if hits:
            found[str(path)] = hits
    return found


def _count_references(directory: Path, root: Path = REPO_ROOT) -> dict[str, int]:
    """Every tracked file naming a path inside `directory`, and how often.

    `git grep` rather than a walk: it reads what git tracks, which is the
    population a rename has to keep correct. A file that is not tracked cannot
    be broken by a commit.
    """
    rel = directory.relative_to(root).as_posix()
    try:
        out = subprocess.run(
            ['git', 'grep', '-c', '-F', f'{rel}/', '--', *SEARCHED],
            cwd=str(root),
            capture_output=True,
            text=True,
            check=False,
        ).stdout
    except OSError:
        return {}
    counts: dict[str, int] = {}
    for line in out.splitlines():
        name, _, count = line.rpartition(':')
        if name and count.isdigit():
            counts[name] = int(count)
    return counts


def plan_move(source: str, package: str, root: Path = REPO_ROOT) -> Plan:
    """Work out everything the move will touch, without touching any of it.

    Read this before moving anything. The three numbers it reports are the
    three ways a move goes wrong: a file that cannot be imported after it
    lands, a sibling import that stops resolving, and a reference nobody
    updated.
    """
    directory = root / source
    if not directory.is_dir():
        raise FileNotFoundError(f'{source}: not a directory')

    files = _python_files(directory)
    target_root = root / 'scripts' / 'le' / package

    moves = []
    for path in files:
        relative = path.relative_to(directory)
        new_name = module_name(path.name) + '.py'
        target = target_root / relative.parent / new_name
        moves.append(Move(source=path, target=target, renamed=new_name != path.name))

    return Plan(
        package=package,
        moves=moves,
        sibling_imports=_find_sibling_imports(files, _sibling_modules(files)),
        references=_count_references(directory, root),
    )
