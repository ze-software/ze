#!/usr/bin/env python3
"""Ensure the repo's tmp/ and cache/ symlinks point at their out-of-tree targets.

    tmp/   -> $TMPDIR/ze/<checkout-id>   (fallback /tmp/ze/<checkout-id>)  disposable scratch
    cache/ -> ${XDG_CACHE_HOME:-~/.cache}/ze                              durable cache

The real directories live OUTSIDE the checkout so the working tree holds only symlinks and
`rm -rf tmp` is always safe.

Contract:
  - Idempotent and safe to call from make prerequisites, hooks, and scripts.
  - Per-checkout scratch: <checkout-id> derives from `git rev-parse --show-toplevel`, so
    every worktree/checkout gets its own scratch (never key on --git-common-dir).
  - NEVER converts an existing REAL tmp/ directory in place (never-destroy-work +
    concurrent-session safety). It only creates a symlink when the path is absent or is
    already a symlink. Converting a populated real tmp/ is `make ze-scratch-migrate`'s job.
  - The durable cache root matches Go's resolveCacheDir() (internal/appliance/cache.go:47-57).
"""

from __future__ import annotations

import hashlib
import os
import shutil
import subprocess
import sys
from pathlib import Path


def repo_root() -> Path:
    """The working-tree root of THIS checkout (per-worktree, not the shared git dir)."""
    out = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=False,
    )
    if out.returncode == 0 and out.stdout.strip():
        return Path(out.stdout.strip()).resolve()
    # Fallback for a non-git tarball: the script lives at <root>/scripts/dev/ensure-links.py.
    return Path(__file__).resolve().parents[2]


def checkout_id(root: Path) -> str:
    """Stable per-checkout key: <basename>-<sha256(abspath)[:16]>.

    The hash keeps two checkouts named 'main' distinct; the basename keeps the path
    human-readable. Keyed on the working-tree path so each worktree gets its own scratch.
    """
    digest = hashlib.sha256(str(root).encode()).hexdigest()[:16]
    return f"{root.name}-{digest}"


def scratch_target(root: Path) -> Path:
    base = os.environ.get("TMPDIR") or "/tmp"
    return Path(base).joinpath("ze", checkout_id(root))


def cache_target() -> Path:
    """Match resolveCacheDir() in internal/appliance/cache.go:47-57."""
    xdg = os.environ.get("XDG_CACHE_HOME")
    if xdg:
        return Path(xdg) / "ze"
    return Path.home() / ".cache" / "ze"


def ensure_symlink(link: Path, target: Path, auto_repoint: bool = True) -> str:
    """Create/repoint `link` -> `target`, creating `target` if absent. Returns a status line.

    Refuses to clobber a real (non-symlink) path at `link`.

    auto_repoint=False means: NEVER silently repoint an existing symlink; report
    the mismatch and leave it. That is for cache/, whose target is derived from
    HOME/XDG rather than from the checkout path. tmp/ is keyed on the checkout
    path, so a mismatch there is real drift (the checkout moved) and must be
    followed; cache/ cannot drift for a given user, so a mismatch means
    ensure-links is running somewhere that does not own this checkout, and
    repointing hijacks the link for whoever does.

    It did, twice. `make ze-qemu-*-test` runs `make ze-unit-test-cached` INSIDE
    the VM with HOME=/root (scripts/evidence/qemu-run.py) over a read-write 9p
    mount, so every QEMU run repointed the host's cache/ to /root/.cache/ze --
    which breaks the host build outright now that GOCACHE lives under cache/
    (Makefile:17). An earlier attempt to detect this by asking whether the
    CURRENT target is a live directory does NOT work: from inside the VM the
    host's target genuinely does not exist, so the probe says "dangling" and
    repoints anyway. There is no reliable ownership signal to infer, so the
    decision is explicit: `ensure-links.py --repoint-cache`.
    """
    if link.is_symlink():
        current = os.readlink(link)
        if current == str(target):
            target.mkdir(parents=True, exist_ok=True)
            return f"ok       {link.name} -> {target}"
        if not auto_repoint:
            # Do NOT stat `current`: an unreadable target (e.g. /root/.cache/ze
            # as a normal user) makes Path.is_dir() raise PermissionError on
            # Python 3.12 rather than return False, which crashed this script.
            return (
                f"MISMATCH {link.name} -> {current} (expected {target}); left as is. "
                f"If this checkout is yours, run: python3 scripts/dev/ensure-links.py --repoint-cache"
            )
        target.mkdir(parents=True, exist_ok=True)
        link.unlink()
        link.symlink_to(target)
        return f"repointed {link.name} -> {target}"

    target.mkdir(parents=True, exist_ok=True)

    if link.exists():
        return (
            f"SKIP     {link.name}: a real path exists here; "
            f"run `make ze-scratch-migrate` to convert it to a symlink"
        )

    try:
        link.symlink_to(target)
    except FileExistsError:
        # Lost a race with a concurrent ensure-links; the winner's link is fine.
        return f"ok       {link.name} -> {target} (created concurrently)"
    return f"created  {link.name} -> {target}"


def first_undeletable_dir(root: Path) -> Path | None:
    """The first directory under `root` whose children cannot be unlinked, or None.

    Unlinking a file needs write AND execute on its PARENT directory, never any
    permission on the file itself. So this is the question a mover has to ask
    before it starts, and asking about readability instead answers a different
    one: a 0555 directory reads and searches perfectly and refuses every unlink.

    It exists because `migrate` below has no undo. Across devices
    `shutil.move` is copytree-then-rmtree, so a failure partway leaves entries
    moved, one entry copied AND half-deleted, and the rest in place, with
    nothing recording how far it got. The realistic cause is not exotic: `go`
    writes gomodcache directories mode 0555 on purpose, so a scratch tree that
    has ever held a module cache cannot be moved across devices at all. Two
    runs died that way on 2026-08-25, each taking every session's
    tmp/commit-session-id-* with it.

    Returned as a path rather than a bool so the refusal can name the directory
    the operator has to fix.
    """
    for path in root.rglob("*"):
        if not path.is_dir() or path.is_symlink():
            continue
        if not os.access(path, os.W_OK | os.X_OK):
            return path.relative_to(root)
    return None


def migrate(link: Path, target: Path, auto_repoint: bool = True) -> str:
    """One-time cutover: convert an existing REAL dir at `link` into a symlink -> `target`.

    Moves the real directory's contents into the target, then replaces it with a symlink.
    Refuses rather than clobber if a name already exists in the target. A symlink or an
    absent path needs no migration and is handled by ensure_symlink(), which is where
    auto_repoint applies; it is accepted here only so both actions share a call site.
    """
    if link.is_symlink() or not link.exists():
        return ensure_symlink(link, target, auto_repoint)
    if not link.is_dir():
        return f"REFUSE   {link.name}: exists and is not a directory; resolve manually"

    undeletable = first_undeletable_dir(link)
    if undeletable is not None:
        return (
            f"REFUSE   {link.name}: {undeletable} is not writable and searchable, "
            f"so its contents cannot be unlinked; nothing was moved. "
            f"A Go module cache is the usual cause: `go` writes gomodcache "
            f"directories mode 0555 on purpose. Run the download that made it "
            f"with GOFLAGS=-modcacherw, or remove the cache, then retry"
        )

    target.mkdir(parents=True, exist_ok=True)
    moved = 0
    for entry in sorted(link.iterdir()):
        dest = target / entry.name
        if dest.exists():
            return (
                f"REFUSE   {link.name}: {dest.name} already exists in the target; "
                f"resolve manually (moved {moved} so far)"
            )
        shutil.move(str(entry), str(dest))
        moved += 1
    link.rmdir()
    link.symlink_to(target)
    return f"migrated {link.name}: moved {moved} entries -> {target}; now a symlink"


# Keep this content byte-for-byte in sync with tmp/go.mod (the tracked sentinel).
SENTINEL = """\
// Sentinel module: marks a REAL tmp/ as a nested module so `go list ./...` and
// `go test ./...` skip the Go/QEMU caches under it (they hold foreign go.mod files
// that would otherwise fail with "directory ... outside main module").
//
// Committed so it is present on a fresh checkout. scripts/dev/ensure-links.py recreates
// it whenever tmp/ is a real directory; after the opt-in `make ze-scratch-migrate`, tmp/
// is a symlink that `go list` skips without any sentinel, so this file is not needed there.
// Keep this content in sync with SENTINEL in scripts/dev/ensure-links.py.
module ze-tmp-scratch

go 1.25
"""


def ensure_sentinel(root: Path) -> None:
    """Keep `go list ./...` out of the caches when tmp/ is a REAL directory.

    A symlinked tmp/ is skipped by `go list` with no sentinel, so this only writes when tmp/
    is a real dir and the file is missing (e.g. just cleared by `make clean-all`).
    """
    tmp = root / "tmp"
    if tmp.is_symlink() or not tmp.is_dir():
        return
    gomod = tmp / "go.mod"
    if not gomod.exists():
        gomod.write_text(SENTINEL)


def main(argv: list[str]) -> int:
    quiet = "--quiet" in argv
    do_migrate = "--migrate" in argv
    repoint_cache = "--repoint-cache" in argv
    root = repo_root()
    action = migrate if do_migrate else ensure_symlink
    # cache/ never auto-repoints: its target is HOME/XDG-derived, so a mismatch
    # means "not my checkout" (the QEMU VM runs make with HOME=/root over a
    # read-write 9p mount), never drift. See ensure_symlink.
    results = [
        action(root / "tmp", scratch_target(root)),
        action(root / "cache", cache_target(), repoint_cache),
    ]
    ensure_sentinel(root)
    noisy = ("SKIP", "REFUSE", "MISMATCH")
    flagged = any(r.startswith(noisy) for r in results)
    if not quiet or flagged or do_migrate:
        for line in results:
            stream = sys.stderr if line.startswith(noisy) else sys.stdout
            print(line, file=stream)
    return 1 if any(r.startswith("REFUSE") for r in results) else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
