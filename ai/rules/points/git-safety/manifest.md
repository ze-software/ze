---
title: Git Safety
when: before any git operation, and when writing or running a commit script
severity: blocking
---
directives ## Directives
  pointer-to-the-git-safety-rationale
commit-rules ## Commit Rules
  the-banned-git-verbs-live-in-instructions-md
  shard-a-shared-plan-log-so-each-session-owns-a-file
  keep-each-shard-single-writer
  what-to-do-about-a-cross-committed-plan-file
  a-cross-commit-is-structural-not-misconduct
  treat-an-explicit-commit-request-as-a-fast-path
  keep-the-tracked-build-check-exemption
  commit-workflow
  git-verbs-inside-the-commit-script-are-allowed
  verify-a-file-is-tracked-before-git-rm
  commit-helper-invocation-examples
  never-put-a-backtick-in-a-commit-helper-body
  the-commit-helper-flags-that-matter
  the-generated-script-has-this-shape
  never-suggest-or-ask-about-committing
pushing-2026-08-05-owner-amendment ## Pushing (2026-08-05, owner amendment)
  push-only-by-owner-order-through-the-commit-helper
  why-the-amendment-and-what-to-do-when-a-push-goes-wrong
  why-the-push-ban-was-amended
  the-hook-forces-every-push-through-the-script
  what-to-do-when-a-push-goes-wrong
commit-granularity ## Commit Granularity
  make-one-commit-per-logical-change
  land-each-chunk-as-it-finishes
  a-wide-working-tree-is-the-signal
verify-a-commit-not-the-working-tree ## Verify a Commit, Not the Working Tree
  the-gate-runs-on-a-commit-in-a-worktree
  an-edit-during-a-run-voids-it
  verify-periodically-not-per-commit
worktree-cleanup ## Worktree Cleanup
  remove-a-worktree-once-its-commits-are-preserved
  find-a-worktree-git-can-no-longer-see
commit-ownership-in-parallel-sessions-2026-07-10-owner ## Commit Ownership in Parallel Sessions (2026-07-10, owner decision)
  clear-the-index-after-a-failed-commit
  read-the-staged-list-after-any-failed-commit
  commit-your-own-work-and-scope-every-script
  include-a-foreign-edit-only-when-it-is-in-scope
  sweep-foreign-non-code-files-never-foreign-source
  a-worktree-is-for-reading-never-for-working
branch-changes-are-forbidden ## Branch Changes Are Forbidden
  stay-on-the-branch-you-started-on
  the-forbidden-branch-changing-commands
  ask-the-user-to-move-or-integrate-a-branch
before-destructive-actions ## Before Destructive Actions
  never-run-a-destructive-git-verb
  save-a-patch-then-stop-before-anything-destructive
forbidden-raw-output ## Forbidden Raw Output
  summarise-git-status-never-dump-it-raw
branch-integration ## Branch Integration
  integrate-a-worktree-branch-by-rebase-not-merge
rebase-onto-diverged-main-driving-the-bookkeeping-conflicts ## Rebase Onto Diverged main: driving the bookkeeping conflicts
  drive-a-diverged-rebase-by-regenerating-bookkeeping
  finish-the-rebase-before-recomputing-derived-ratchets
  rebase-continue-reports-a-misleading-conflict-error
gpg-signing ## GPG Signing
  never-disable-gpg-signing-or-a-hook
  unlock-the-gpg-agent-when-signing-fails
codeberg-cli ## Codeberg CLI
  use-tea-for-pull-requests-and-issues
