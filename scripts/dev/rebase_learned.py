#!/usr/bin/env python3
"""Drive an in-progress rebase that keeps re-conflicting on the learned-index
bookkeeping files, auto-resolving only what is mechanically derivable and
halting on anything that needs judgment.

WHY THIS EXISTS
    Rebasing a local branch onto a diverged origin/main routinely stops on a
    conflict in the file every learned-summary commit regenerates:

      - ai/LEARNED-FULL-INDEX.md        (fully generated from the *.md files)

    It is DERIVABLE, so resolving it by hand on every one of ~20 commits is
    pure toil. This tool resolves it the only correct way at each stop:

      - LEARNED-FULL-INDEX -> regenerate with scripts/dev/learned_index.py

    Everything else is a judgment call, so the tool STOPS and reports rather
    than guessing. See ai/rules/git-safety.md "Rebase Onto Diverged main:
    driving the bookkeeping conflicts" for the surrounding procedure (finish
    the rebase first, renumber colliding entries afterwards).

USAGE
    # Start the rebase yourself (this tool never starts/aborts one):
    #   git rebase origin/main        # stops on the first conflict
    python3 scripts/dev/rebase_learned.py [FLAGS]

    Re-run after each STOP once you have decided how to resolve the reported
    file; pass the matching flag so the tool applies it and drives on.

FLAGS (all judgment-driven, all logged, none silent)
    --accept-incoming-delete
        Accept modify/delete conflicts where the replayed commit DELETES the
        file (e.g. a closure commit that `git rm`s a spec an earlier commit
        modified). In a rebase the replayed commit is always one of ours, so
        its deletion is the intended outcome. Applies to every such conflict
        for the rest of the run.
    --take-theirs PATH      (repeatable)
        Resolve a content conflict on PATH by taking the replayed commit's
        version wholesale (git show :3:). Re-applies whenever PATH conflicts.
    --take-ours PATH        (repeatable)
        Same, taking HEAD's version (git show :2:).

EXIT CODES
    0  rebase complete (or none in progress)
    2  an underlying git command failed unexpectedly
    3  a real conflict needs judgment (file + commit reported)
    4  `git rebase --continue` failed for an unhandled reason (state dumped)
    5  hit the MAX_ITERS safety guard
    6  unstaged tracked changes block --continue (git's "edit all merge
       conflicts" message is MISLEADING; the real cause is listed)

NOTES
    - git add is always scoped to specific paths; never `git add -A`.
    - Untracked files (scratch scripts) never block --continue.
    - This tool runs git subprocesses; per ai/rules/git-safety.md the human
      starts the rebase and this bundled script performs the resolution steps.
"""

import os
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
INDEX = "ai/LEARNED-FULL-INDEX.md"
BOOKKEEPING = {INDEX}
MAX_ITERS = 200

ENV = dict(os.environ, GIT_EDITOR="true", GIT_SEQUENCE_EDITOR="true")


def parse_args():
    accept_delete = False
    take_theirs, take_ours = [], []
    it = iter(sys.argv[1:])
    for a in it:
        if a in ("-h", "--help"):
            print(__doc__)
            sys.exit(0)
        elif a == "--accept-incoming-delete":
            accept_delete = True
        elif a == "--take-theirs":
            take_theirs.append(next(it))
        elif a == "--take-ours":
            take_ours.append(next(it))
        else:
            print(f"unknown arg: {a}\n\n{__doc__}")
            sys.exit(2)
    return accept_delete, take_theirs, take_ours


def git(*args, check=False):
    r = subprocess.run(
        ["git", *args],
        cwd=ROOT,
        env=ENV,
        capture_output=True,
        text=True,
    )
    if check and r.returncode != 0:
        print(f"git {' '.join(args)} FAILED ({r.returncode})")
        print(r.stdout)
        print(r.stderr)
        sys.exit(2)
    return r


def rebase_in_progress():
    for name in ("rebase-merge", "rebase-apply"):
        p = git("rev-parse", "--git-path", name).stdout.strip()
        if (ROOT / p).exists():
            return True
    return False


def unmerged_paths():
    r = git("diff", "--name-only", "--diff-filter=U")
    return [ln for ln in r.stdout.splitlines() if ln.strip()]


def unstaged_tracked():
    # Unstaged working-tree changes to tracked files. `git rebase --continue`
    # refuses when ANY exist and prints the MISLEADING "You must edit all merge
    # conflicts" message (builtin/rebase.c ACTION_CONTINUE checks
    # has_unstaged_changes(), not just unmerged entries). Untracked files
    # (scratch scripts) do not count.
    r = git("diff", "--name-only")
    return [ln for ln in r.stdout.splitlines() if ln.strip()]


def conflict_stages(path):
    """Set of merge stages present for a conflicted path: 1=base, 2=ours (HEAD
    rebased so far), 3=theirs (the commit being replayed)."""
    r = git("ls-files", "-u", "--", path)
    stages = set()
    for ln in r.stdout.splitlines():
        # <mode> <sha> <stage>\t<path>
        head = ln.split("\t", 1)[0].split()
        if len(head) == 3:
            stages.add(int(head[2]))
    return stages


def is_incoming_delete(path):
    """True when the commit being replayed DELETES this path (no stage 3) while
    our side still has it (stage 2)."""
    st = conflict_stages(path)
    return 3 not in st and 2 in st


def take_side(path, stage):
    """Resolve a content conflict by taking one merge stage wholesale:
    2=ours (HEAD rebased so far), 3=theirs (commit being replayed). Uses
    `git show :N:` (read-only) instead of hand-editing, so a spec/design edit
    hook does not fire on a mechanical merge resolution."""
    r = git("show", f":{stage}:{path}")
    if r.returncode != 0:
        print(f"take_side stage={stage} {path} FAILED:\n{r.stderr}")
        sys.exit(2)
    (ROOT / path).write_text(r.stdout, encoding="utf-8")
    git("add", "--", path, check=True)


def regen_index():
    r = subprocess.run(
        [sys.executable, "scripts/dev/learned_index.py"],
        cwd=ROOT,
        env=ENV,
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        print("learned_index.py FAILED:")
        print(r.stdout)
        print(r.stderr)
        sys.exit(2)


def current_commit_desc():
    p = ROOT / git("rev-parse", "--git-path", "rebase-merge").stdout.strip()
    msg = p / "message"
    if msg.exists():
        text = msg.read_text(encoding="utf-8")
        return text.splitlines()[0] if text.strip() else ""
    return ""


def main():
    accept_delete, take_theirs, take_ours = parse_args()

    if not rebase_in_progress():
        print("No rebase in progress. Nothing to do.")
        return 0

    for it in range(1, MAX_ITERS + 1):
        if not rebase_in_progress():
            print(f"\n=== REBASE COMPLETE (after {it - 1} resolution step(s)) ===")
            return 0

        um = unmerged_paths()
        if um:
            extra = [f for f in um if f not in BOOKKEEPING]

            # Explicit per-file content resolutions (judgment-driven).
            for f in list(extra):
                if f in take_theirs:
                    take_side(f, 3)
                    print(f"[iter {it}] take theirs (stage 3) -> {f}")
                    extra.remove(f)
                elif f in take_ours:
                    take_side(f, 2)
                    print(f"[iter {it}] take ours (stage 2) -> {f}")
                    extra.remove(f)

            # Accept incoming deletions (our closure commits that `git rm` a
            # spec an earlier commit modified). Opt-in; each removal is logged.
            if extra and accept_delete:
                still = []
                for f in extra:
                    if is_incoming_delete(f):
                        git("rm", "--", f, check=True)
                        print(f"[iter {it}] accept incoming delete -> git rm {f}")
                    else:
                        still.append(f)
                extra = still

            if extra:
                print("\n*** STOP: real conflict, not just bookkeeping ***")
                print(f"Unmerged files: {um}")
                print(f"Non-bookkeeping (need judgment): {extra}")
                print(f"Applying commit: {current_commit_desc()!r}")
                print(
                    "Resolve, then re-run with --take-theirs/--take-ours/"
                    "--accept-incoming-delete as appropriate."
                )
                return 3

            # Resolve the derivable bookkeeping file.
            added = []
            if INDEX in um:
                regen_index()
                added.append(INDEX)
                print(f"[iter {it}] regenerated {INDEX}")
            if added:
                git("add", *added, check=True)

        # Unstaged changes to tracked files block --continue with a MISLEADING
        # "edit all merge conflicts" message. Surface the real cause clearly.
        dirty = unstaged_tracked()
        if dirty:
            print("\n*** STOP: unstaged changes block 'git rebase --continue' ***")
            print("git will report 'You must edit all merge conflicts' but the")
            print("real cause is these unstaged tracked files. Stage, revert, or")
            print("set them aside, then re-run:")
            for f in dirty:
                print(f"  {f}")
            return 6

        # Continue (works both after a resolution and when stopped on empty).
        r = git("rebase", "--continue")
        out = r.stdout + r.stderr
        if r.returncode == 0:
            print(f"[iter {it}] continue OK")
            continue

        low = out.lower()
        if (
            "is now empty" in low
            or "nothing to commit" in low
            or "did you forget to use 'git add'" in low
            or "patch is empty" in low
        ):
            print(f"[iter {it}] commit became empty -> skip")
            git("rebase", "--skip", check=True)
            continue

        if unmerged_paths():
            # New conflict surfaced by --continue advancing to next commit.
            print(f"[iter {it}] continue surfaced new conflicts, re-loop")
            continue

        print("\n*** STOP: unexpected 'git rebase --continue' failure ***")
        print("--- continue output ---")
        print(out)
        print("--- git status --short ---")
        print(git("status", "--short").stdout)
        print("--- git ls-files -u (unmerged index) ---")
        print(git("ls-files", "-u").stdout or "(none)")
        return 4

    print("\n*** STOP: hit MAX_ITERS guard ***")
    return 5


if __name__ == "__main__":
    sys.exit(main())
