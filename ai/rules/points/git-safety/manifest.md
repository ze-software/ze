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
commit-ownership-in-parallel-sessions-2026-07-10-owner ## Commit Ownership in Parallel Sessions (2026-07-10, owner decision)
  clear-the-index-after-a-failed-commit
  read-the-staged-list-after-any-failed-commit
  commit-your-own-work-and-scope-every-script
  include-a-foreign-edit-only-when-it-is-in-scope
  sweep-foreign-non-code-files-never-foreign-source
before-any-commit ## Before Any Commit
  step-0-does-ze-verify-apply
  run-verify-only-when-the-commit-can-affect-the-build
  which-file-types-require-ze-verify
  run-verify-when-any-file-in-a-mixed-commit-needs-it
  step-1-if-ze-verify-applies-blocking
  run-make-ze-verify-and-check-freshness-first
  the-pre-commit-verify-checklist
  structural-gates-are-never-known-red-blocking
  never-park-a-deterministic-structural-gate
  clear-a-broken-head-by-committing-the-producer
  structural-red-ok-is-an-owner-only-escape
  a-stale-generated-file-is-structural-not-flaky
  your-working-tree-is-not-what-you-committed-blocking
  only-one-check-compiles-what-git-holds
  what-ze-tracked-build-check-reads
  commit-the-producer-with-its-consumer
  bisect-a-broken-head-with-rev
  the-tracked-build-check-never-reads-test-files
  the-doc-test-only-checks-that-escape-the-gate
  keep-the-structural-gate-list-matching-live-stages
  the-helper-refuses-a-commit-while-a-gate-is-red
  thomas-owner-override-commit-without-verify
  thomas-can-override-the-verify-requirement
  the-override-needs-both-parts-said-explicitly
  the-two-parts-the-override-must-name
  phrases-that-activate-the-owner-override
  when-the-override-is-active
  what-the-override-permits-and-forbids
  known-red-full-verify-scope-to-changed-blocking
  scope-the-gate-to-changes-when-verify-is-known-red
  the-scoped-gates-to-run-instead
  list-only-your-own-files-in-the-commit-script
  attribute-every-red-before-scoping-around-it
  never-let-a-red-persist-across-sessions
  concurrent-verify-runs-blocking
  run-one-verify-at-a-time-repo-wide
  what-to-do-while-another-verify-holds-the-lock
  the-verify-lock-releases-when-the-command-exits
  running-ze-verify
  run-verify-in-the-foreground-and-wait
  a-shared-checkout-never-gives-a-clean-ze-verify-blocking
  a-clean-full-verify-is-unreachable-in-a-shared-tree
  a-full-gate-run-reddens-other-sessions
  use-evidence-scoped-to-your-own-files
  the-scoped-evidence-a-shared-checkout-owes
  unverified-is-correct-in-a-shared-checkout
  green-every-structural-gate-even-in-a-shared-tree
  never-edit-the-tree-while-a-verify-runs
  once-at-the-end-never-during-development-blocking
  run-the-full-gate-once-when-the-work-is-finished
  run-the-target-that-owns-what-you-changed
  go-through-make-or-carry-gocache-yourself
  test-one-package-with-ze-test-pkg
  ze-test-pkg-examples
  which-target-owns-each-surface
  derive-the-owning-target-when-the-table-has-no-row
  read-the-whole-failure-summary-before-re-running
  traps-that-misread-a-verify-log
  a-second-verify-blocks-rather-than-overlaps
  give-a-narrow-failure-a-narrow-re-run
  plan-the-first-full-run-to-be-the-last
  do-not-stop-to-ask-which-way
  step-2-always
  the-spec-completion-and-report-checklist
  never-commit-with-lint-issues-or-no-test-evidence
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
  drive-a-diverged-rebase-with-rebase-learned-py
  finish-the-rebase-before-fixing-learned-numbers
  rebase-continue-reports-a-misleading-conflict-error
gpg-signing ## GPG Signing
  never-disable-gpg-signing-or-a-hook
  unlock-the-gpg-agent-when-signing-fails
codeberg-cli ## Codeberg CLI
  use-tea-for-pull-requests-and-issues
