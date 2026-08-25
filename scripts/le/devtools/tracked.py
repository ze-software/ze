"""Does `le` work from what GIT holds, rather than from the working tree?

Those are two different trees, and only one of them is what anybody else gets.

This exists because they diverged and nothing said so. On 2026-08-25 a clean
`git archive HEAD` of `scripts/le` failed to load 21 of 21 areas: 19 on
`ImportError: cannot import name 'stream' from 'le.process'` and 2 on modules
that were never added. `le` and every make target forwarding to it were dead at
HEAD, for every session and for CI, while the working tree ran perfectly.

**Three commits produced it and not one of them was wrong.** `gate.py` imports
`stream` from `le.process`. The commit ADDING `stream` was held back by a
per-commit file another session was holding. The commit USING it had no such
dependency and went in. So HEAD carried the caller without the callee, and the
two commits looked independent because the file coupling them was a third one
neither of them named.

Every one of those commits was verified. All of them were verified against the
WORKING TREE, where every file was present. The tree was never the thing that
had to work.

Go has not had this failure, and the reason is
`ze-repository-tracked-build-check`: it compiles what git holds. Python had no
counterpart. This is that counterpart, and it is deliberately the cheapest
useful form -- import every registered area and see whether it loads. An
unimportable area is the failure that actually happened, and it is the one a
reference to a missing module always produces.
"""

from __future__ import annotations

import shutil
import subprocess
import tempfile
from dataclasses import dataclass
from pathlib import Path

from le.paths import REPO_ROOT

__all__ = ['Verdict', 'check_tracked_import', 'export_tracked']

# What `le` needs from git to be `le`: the package and the entry-point shim.
TRACKED_PATHS = ('scripts/le', 'le')

# The probe. It runs INSIDE the exported tree, in its own interpreter, because
# this process has already imported the working tree's `le` and would answer
# about that one.
PROBE = """
import sys
sys.path.insert(0, 'scripts')
from le.registry import REGISTRY
broken = []
for entry in REGISTRY:
    try:
        entry.load()
    except BaseException as err:
        broken.append(f'{entry.name}: {type(err).__name__}: {err}')
if not REGISTRY:
    print('NO-AREAS')
    raise SystemExit(3)
for line in broken:
    print(line)
raise SystemExit(1 if broken else 0)
"""


@dataclass(frozen=True)
class Verdict:
    """What the committed tree said."""

    ok: bool
    areas: int
    broken: tuple[str, ...]
    detail: str = ''


def export_tracked(revision: str, destination: Path) -> None:
    """Write what git holds at `revision` into `destination`.

    `git archive` rather than a copy: it reads the object store, so a file
    present in the working tree and absent from the commit does not come along.
    That difference is the entire point.
    """
    archive = subprocess.run(
        ['git', 'archive', revision, *TRACKED_PATHS],
        cwd=str(REPO_ROOT),
        capture_output=True,
        check=False,
    )
    if archive.returncode != 0:
        raise RuntimeError(archive.stderr.decode(errors='replace').strip() or 'git archive failed')
    extract = subprocess.run(
        ['tar', '-x', '-C', str(destination)],
        input=archive.stdout,
        capture_output=True,
        check=False,
    )
    if extract.returncode != 0:
        raise RuntimeError(extract.stderr.decode(errors='replace').strip() or 'tar failed')


def check_tracked_import(revision: str = 'HEAD') -> Verdict:
    """Load every registered area from `revision`, and say which do not.

    A fresh interpreter in a fresh directory. Importing here would answer about
    the working tree, which is the tree already known to work and the one whose
    verdict nobody needs.
    """
    workspace = Path(tempfile.mkdtemp(prefix='le-tracked-'))
    try:
        try:
            export_tracked(revision, workspace)
        except RuntimeError as why:
            return Verdict(ok=False, areas=0, broken=(), detail=str(why))

        if not (workspace / 'scripts' / 'le' / 'registry.py').is_file():
            return Verdict(
                ok=False,
                areas=0,
                broken=(),
                detail=f'{revision} holds no scripts/le/registry.py',
            )

        result = subprocess.run(
            ['python3', '-c', PROBE],
            cwd=str(workspace),
            capture_output=True,
            text=True,
            check=False,
        )
        lines = tuple(line for line in result.stdout.splitlines() if line.strip())

        if result.returncode == 3 or lines == ('NO-AREAS',):
            # A registry that lists nothing would make every assertion below
            # vacuous, and is itself the kind of breakage this looks for.
            return Verdict(ok=False, areas=0, broken=(), detail='the committed registry is empty')

        areas = _registered(workspace)
        return Verdict(ok=result.returncode == 0, areas=areas, broken=lines)
    finally:
        shutil.rmtree(workspace, ignore_errors=True)


def _registered(workspace: Path) -> int:
    """How many areas the committed registry declares, counted in that tree."""
    counted = subprocess.run(
        [
            'python3',
            '-c',
            "import sys; sys.path.insert(0,'scripts')\n"
            'from le.registry import REGISTRY; print(len(REGISTRY))',
        ],
        cwd=str(workspace),
        capture_output=True,
        text=True,
        check=False,
    )
    text = counted.stdout.strip()
    return int(text) if text.isdigit() else 0
