# Removal leaves the file on disk

A command removes a path from the tree it commits and does not remove it from
the working tree. The commit is right, `git log` is right, and the file is still
there, now untracked. Every gate that answers a question by asking the
FILESYSTEM then reads the old tree, and the one that reads git reads the new
one, so the two disagree with nothing saying which is current.

The tell is a gate that goes green over a removal it should have noticed, and a
`git status` carrying a `??` line for a path the last commit deleted.

`/ze-close` names the shape from the other side: `find_dangling` resolves a
citation with a filesystem test, "which sees an untracked file, so the working
tree reads green while the tree your commit produces is red". The same test that
hides a dangling citation hides the removal that created it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-21 | - | the verify worktree sweeper, `sweepAbandoned` and `preservedLine` (`internal/le/verify/cleanup.go`) | Two 2026-10-18 worktrees, removal interrupted (about 10k deletions each, 0 modified, 0 untracked, HEAD on main, 1.6G), were kept as "holds uncommitted changes" for three days: deletions alone count as dirt | open: removed by hand. Fix: a dead run's worktree whose only dirt is deletions and whose HEAD is on a branch is an interrupted removal; finish it |
| 2026-10-17 | - | `installBytes` (`internal/component/config/storage/tree.go`), found reviewing `4c1db99ae2` | A crash between staging and rename leaves a `.ze-storage-*` frame beside `database/`; the deferred unlink never runs and nothing removed one. A plain prefix sweep would race an offline tree's or a loose config's live write in the same folder. | fixed: Fixed: a frame names the tree whose owner lock covers it; `WriteConfigFile` now publishes only through the owning store. `sweepStages` removes that tree's leftovers on its writable open, under the lock, best-effort. |
| 2026-10-07 | remove-takes-the-working-tree-copy | `renderWorkingTreeRemoval` and the capture in `renderPrivateIndex` (`internal/le/commit/script.go`) | The repair the rows below asked for. A fifth leak was live: `plan/spec-password-weakness-warning.md` (`b080fe755e`, identical blob), left for the owner | fixed: Fixed: after the commit, a copy is deleted only when its staged entry, mode included, equals HEAD's; any other copy is kept and named. The `WriteAtomic` row stays open |
| 2026-10-07 | feature-maturity-declared | `WriteAtomic` (`internal/le/derived/derived.go`) | A killed process leaves its temporary beside the target: `plan/.roadmap.md.tmp-1095422799` sits in the tree. The temporary is removed only by the deferred error branch, which a SIGKILL or a crash never runs, and nothing sweeps a stale `.tmp-*` on the next write. | fixed: Fixed: `sweepStaleTemporaries` removes a regular `.<base>.tmp-*` older than `staleTemporaryAge` on the next write of that target. |
| 2026-09-19 | - | the verify worktree sweeper, `sweepAbandoned` (`internal/le/verify/cleanup.go`) | An `.owner` marker outlives its worktree directory and is never swept, because the sweep skips a non-directory, and the sweeper runs only inside a full `./le verify worktree`, so reclaiming one orphan meant a whole verification | fixed: 7c96f64db1 sweeps a dead owner's directoryless `.owner` marker and registers `./le reclaim worktrees` as a standalone entry (`TestStandaloneReclaimWorktrees`) |
| 2026-09-12 | derived-indexes-answer-a-query | the same `renderSharedIndexRepair` path (`internal/le/commit/script.go`), reached through `/ze-close` commit B | Fourth instance: closure commit `6542cc081` removed `plan/spec-derived-indexes-answer-a-query.md` from git and left a byte-identical untracked copy: the generated script only ran `git update-index --force-remove` | fixed: d70bbec7fc renderWorkingTreeRemoval deletes the working-tree copy that matches the committed blob; TestARemovalDeletesTheWorkingTreeCopyItCommitted, TestAClosureLeavesNoSpecBehind |
| 2026-09-07 | rib-package-dead-surface | the same `renderSharedIndexRepair` path (`internal/le/commit/script.go`), reached through `/ze-close` commit B | Second instance: closure commit `9c3b673ebc` left a byte-identical untracked copy of `plan/spec-rib-package-dead-surface.md`, and `f339bf0ee7` had leaked `plan/spec-firewall-domain-group.md` the same way; no gate reported either | fixed: d70bbec7fc renderWorkingTreeRemoval deletes the working-tree copy that matches the committed blob; TestARemovalDeletesTheWorkingTreeCopyItCommitted, TestAClosureLeavesNoSpecBehind |
| 2026-09-06 | ze-test-dns-stub | `./le commit create ... remove <path>`, through `renderCommitBlock` and `renderSharedIndexRepair` (`internal/le/commit/script.go`) | The generated script removed the path from the private and shared indexes only, so closure commit B left `plan/spec-ze-test-dns-stub.md` on disk and `./le spec citation` answered `OK (322 specs)` over a tree of 321 | fixed: d70bbec7fc renderWorkingTreeRemoval deletes the working-tree copy that matches the committed blob; TestARemovalDeletesTheWorkingTreeCopyItCommitted, TestAClosureLeavesNoSpecBehind |
