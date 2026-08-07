---
kind: note
level:
stage:
rationale: plan/learned/1155-learned-numbers-collide-across-branches.md
---
A rebase of local commits onto a diverged `origin/main` re-conflicts on
`ai/LEARNED-FULL-INDEX.md` at nearly every learned-touching commit -- the
cross-branch learned-number collision covered in "Commit Rules" step 4 and
`plan/learned/1155`. That file is derivable, so drive the rebase with
`scripts/dev/rebase_learned.py`: the human starts (and, if needed, aborts) the
rebase; the script regenerates the index (via `learned_index.py`) at each stop
and HALTS on any other unmerged path. Resolve that file, then re-run with
`--take-theirs PATH` / `--take-ours PATH` / `--accept-incoming-delete` (each
logged, never silent). `--help` documents the flags and exit codes.
