"""Where things are.

One module answers this so no other module computes a repository root from its
own `__file__` depth. A file that moves one directory down otherwise takes a
silently wrong root with it, and the failure appears in whatever that root was
later joined with.
"""

from __future__ import annotations

from pathlib import Path

__all__ = ['REPO_ROOT', 'relative_to_root']

# scripts/le/paths.py -> scripts/le -> scripts -> the checkout.
REPO_ROOT: Path = Path(__file__).resolve().parents[2]


def relative_to_root(path: Path) -> str:
    """`path` written the way a reader would type it, when it is inside the tree.

    Absolute paths in output are noise when every one of them shares the same
    long prefix. A path outside the checkout keeps its absolute form, because
    shortening it would be a lie about where it is.
    """
    try:
        return str(path.relative_to(REPO_ROOT))
    except ValueError:
        return str(path)
