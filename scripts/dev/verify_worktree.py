#!/usr/bin/env python3
"""Run the pre-commit gate against a COMMIT, in a throwaway worktree.

The gate takes 25 to 53 minutes on this hardware (measured 2026-08-21:
1486s, 1574s and 3195s, `tmp/.ze-verify-duration.txt`) and it reads the WORKING
TREE, so running it in place has two costs that compound. Nobody can touch the
tree while it runs, and any edit that lands mid-run silently invalidates
whatever stage had already read the old bytes. The usual response is to batch
work until one big verify is worth the wait, which is the accumulation
`ai/rules/git-safety.md` bans.

Verifying a COMMIT instead removes both. The worktree is a fixed snapshot, so
no edit can invalidate it, and the working tree stays free for the next chunk.

Usage:
    python3 scripts/dev/verify_worktree.py [--commit HEAD] [--keep]

The worktree is removed on every exit path unless --keep is given. Exit status
is the gate's own.
"""

import argparse
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path


def run(cmd: list[str], **kw) -> subprocess.CompletedProcess:
    return subprocess.run(cmd, check=False, text=True, **kw)


def repo_root() -> Path:
    out = run(["git", "rev-parse", "--show-toplevel"], capture_output=True)
    if out.returncode != 0:
        sys.exit("verify-worktree: not inside a git repository")
    return Path(out.stdout.strip())


def resolve(commit: str) -> str:
    out = run(["git", "rev-parse", "--verify", f"{commit}^{{commit}}"], capture_output=True)
    if out.returncode != 0:
        sys.exit(f"verify-worktree: {commit} does not name a commit")
    return out.stdout.strip()


def remove_worktree(root: Path, path: Path) -> None:
    """Drop the worktree and its registration, leaving no prunable entry."""
    run(["git", "-C", str(root), "worktree", "remove", "--force", str(path)],
        capture_output=True)
    if path.exists():
        # `worktree remove` refuses a directory it no longer recognises; the
        # registration still has to go, so clear both by hand.
        shutil.rmtree(path, ignore_errors=True)
    run(["git", "-C", str(root), "worktree", "prune"], capture_output=True)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--commit", default="HEAD", help="commit to verify (default HEAD)")
    parser.add_argument("--target", default="ze-precommit-verify", help="make target to run")
    parser.add_argument("--keep", action="store_true", help="leave the worktree in place")
    args = parser.parse_args()

    root = repo_root()
    sha = resolve(args.commit)
    stamp = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
    path = root / "tmp" / "verify-worktree" / f"{stamp}-{sha[:12]}"
    path.parent.mkdir(parents=True, exist_ok=True)

    print(f"verify-worktree: {sha[:12]} -> {path}")
    add = run(["git", "-C", str(root), "worktree", "add", "--detach", str(path), sha])
    if add.returncode != 0:
        return add.returncode

    try:
        # A worktree has no tmp/ of its own, and the gate writes its logs and
        # its session directory there.
        (path / "tmp").mkdir(exist_ok=True)
        env = dict(os.environ)
        # The gate reads this to decide whether it is nested inside another
        # admitted job. A worktree run is its own job.
        env.pop("ZE_RUN_JOB", None)
        print(f"verify-worktree: make {args.target}")
        result = run(["make", args.target], cwd=str(path), env=env)
        print(f"verify-worktree: {args.target} exit={result.returncode}")
        if result.returncode != 0:
            print(f"verify-worktree: logs under {path}/tmp/verify/")
            if not args.keep:
                print("verify-worktree: re-run with --keep to inspect them")
        return result.returncode
    finally:
        if args.keep:
            print(f"verify-worktree: kept {path}")
        else:
            remove_worktree(root, path)
            print("verify-worktree: worktree removed")


if __name__ == "__main__":
    sys.exit(main())
