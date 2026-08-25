"""Where things are.

One module answers this so no other module counts directories up from its own
`__file__`. That counting is what makes a file unmovable: `parents[2]` in
`scripts/dev/foo.py` means the checkout, and the same expression in
`scripts/le/dev/foo.py` means `scripts/`. Nothing raises. The path is simply
wrong, and a gate reads the wrong tree or writes to the wrong place.

94 of 172 movable files were written that way, which is the real cost of
bringing them under `le`, larger than the renames and larger than the
references.

**`ZE_REPO_ROOT` is the answer, and it already existed.**
`scripts/evidence/qemu-all-tests.sh` exports it as `/workspace` inside a
container, where deriving the root from a file's position is wrong; and
`zeRepoRootEnv` (`internal/test/runner/runner_exec_util.go`) passes it to every
test subprocess. This makes that the general contract rather than two special
cases:

    set        `le` sets it once at startup and exports it, so every gate --
               forked or imported -- inherits one answer
    unset      it is DISCOVERED by walking up for a marker, so a tool run on
               its own still works, from any depth, at any path
    overridden a container, a worktree or an odd layout says so and is obeyed

The discovery is what actually removes the positional dependency. A file can
then move anywhere in the tree and still find the root, whether `le` invoked it
or a person did.
"""

from __future__ import annotations

import os
from pathlib import Path

__all__ = ['REPO_ROOT', 'ROOT_ENV', 'relative_to_root', 'repo_root']

# The variable every tool reads, and the one `le` exports.
ROOT_ENV = 'ZE_REPO_ROOT'

# What identifies a Ze checkout. `go.mod` alone is not enough: a vendored
# module directory has one. The pair is what makes the answer unambiguous, and
# `feature-gates.txt` is Ze's own rather than any Go project's.
MARKERS = ('go.mod', 'feature-gates.txt')


def _discovered(start: Path) -> Path | None:
    """The nearest ancestor of `start` that looks like a Ze checkout."""
    for candidate in (start, *start.parents):
        if all((candidate / marker).exists() for marker in MARKERS):
            return candidate
    return None


def repo_root(*, export: bool = False) -> Path:
    """The checkout this code belongs to.

    `ZE_REPO_ROOT` wins when set, because the environment knows things the
    filesystem cannot: a container that mounted the tree elsewhere, a worktree,
    a test fixture standing in for a checkout.

    Otherwise it is discovered by walking up for the markers, which is what
    lets a moved file keep working. Discovery starts at THIS file rather than
    at the working directory: a gate is run from anywhere, and cwd is not a
    fact about where the code lives.

    `export` writes the answer back into the environment. `le` does that once
    at startup so every child process inherits one root instead of each
    rediscovering it, and so a script that shells out passes it on.
    """
    named = os.environ.get(ROOT_ENV)
    if named:
        return Path(named).resolve()

    found = _discovered(Path(__file__).resolve().parent)
    if found is None:
        # Two directories up from le/paths.py is the historical answer, and it
        # is right for a normal checkout. Reaching here means the markers are
        # gone, which is a stranger problem than a wrong root.
        found = Path(__file__).resolve().parents[2]

    if export:
        os.environ[ROOT_ENV] = str(found)
    return found


# Resolved once at import. Every module that wants the root reads this rather
# than computing its own, so one process holds one answer.
REPO_ROOT: Path = repo_root()


def relative_to_root(path: Path) -> str:
    """`path` written the way a reader would type it, when it is inside the tree.

    Absolute paths in output are noise when every one shares the same long
    prefix. A path outside the checkout keeps its absolute form, because
    shortening it would be a lie about where it is.
    """
    try:
        return str(path.relative_to(REPO_ROOT))
    except ValueError:
        return str(path)
