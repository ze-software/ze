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

The worktree is removed on every exit path unless --keep is given. A red run's
stage logs are copied to `tmp/verify-worktree-logs/` first, so removing the
worktree does not destroy the reason it went red. Exit status is the gate's own.
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
    out = run(
        ["git", "rev-parse", "--verify", f"{commit}^{{commit}}"], capture_output=True
    )
    if out.returncode != 0:
        sys.exit(f"verify-worktree: {commit} does not name a commit")
    return out.stdout.strip()


def worktree_path(root: Path, sha: str, stamp: str) -> Path:
    """Where this run's worktree goes: a NEW directory every time.

    The timestamp is not decoration, and it is not for the operator. `go test`
    keys a cached verdict on the absolute paths its testlog recorded, so a fresh
    directory misses the cache whatever the module and build caches hold, and a
    gate run here cannot answer `ok (cached)` for a run that never happened.

    Measured 2026-08-22 on `./scripts/status/`, one of the exec sites in
    `plan/journal/test-cache-stale.md`: 1.674s then `(cached)` in the main tree,
    2.384s in a fresh worktree at the same sha with that package byte-identical,
    then `(cached)` on a second run inside that same worktree.

    BOTH halves of that matter, and the second is the one a reader loses. The
    stale-cache class does not reach a gate run in a fresh worktree at a new
    path. It DOES reach those same gates run over the shared working tree, where
    the path is stable and a warm verdict outlives an edit to a producer the
    test only execs. That is what `DEBT_GATE_RUNNERS`
    (`scripts/dev/commit_helper.py`) does today, and it is why the debt pass is
    moving here rather than staying there.

    So making this path stable to save the checkout cost would be right about
    the cost and wrong about the consequence: it re-arms that class against the
    artifact that records verification evidence, silently. This is a function
    rather than one line inside `main` so that the property can be ASSERTED
    (`TimestampedPathTest`) instead of only described here. A comment reaches
    whoever reads this line; a test reaches whoever changes it.
    """
    return root / "tmp" / "verify-worktree" / f"{stamp}-{sha[:12]}"


def save_logs(root: Path, path: Path, name: str) -> Path | None:
    """Copy a red run's stage logs out of the worktree before it is removed.

    The gate writes its per-stage logs to `tmp/verify` inside the tree it runs
    in (`stageLogDir`, scripts/status/verify_run.go), so a worktree run's logs
    live inside the worktree and died with it. The run costs 25 to 53 minutes,
    which made re-running the whole gate the only way to read why it went red,
    and `--keep` could only help somebody who had already guessed it would.

    A green run's logs are not saved. An hour of stage output nobody reads
    changes nothing about what the operator does next, which is the same reason
    a passing gate prints one line rather than its whole log.
    """
    src = path / "tmp" / "verify"
    if not src.is_dir():
        return None
    dest = root / "tmp" / "verify-worktree-logs" / name
    dest.parent.mkdir(parents=True, exist_ok=True)
    shutil.rmtree(dest, ignore_errors=True)
    shutil.copytree(src, dest)
    return dest


def remove_worktree(root: Path, path: Path) -> None:
    """Drop the worktree and its registration, leaving no prunable entry."""
    run(
        ["git", "-C", str(root), "worktree", "remove", "--force", str(path)],
        capture_output=True,
    )
    if path.exists():
        # `worktree remove` refuses a directory it no longer recognises; the
        # registration still has to go, so clear both by hand.
        shutil.rmtree(path, ignore_errors=True)
    run(
        ["git", "-C", str(root), "worktree", "prune", "--expire", "now"],
        capture_output=True,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--commit", default="HEAD", help="commit to verify (default HEAD)"
    )
    parser.add_argument(
        "--target", default="ze-precommit-verify", help="make target to run"
    )
    parser.add_argument(
        "--keep", action="store_true", help="leave the worktree in place"
    )
    args = parser.parse_args()

    root = repo_root()
    sha = resolve(args.commit)
    path = worktree_path(root, sha, time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()))
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
            saved = save_logs(root, path, path.name)
            if saved is not None:
                print(f"verify-worktree: logs saved to {saved}")
            else:
                print(
                    "verify-worktree: the gate wrote no stage logs "
                    f"({path}/tmp/verify/ is absent), so it went red before "
                    "the first stage"
                )
        return result.returncode
    finally:
        if args.keep:
            print(f"verify-worktree: kept {path}")
        else:
            remove_worktree(root, path)
            print("verify-worktree: worktree removed")


if __name__ == "__main__":
    sys.exit(main())
