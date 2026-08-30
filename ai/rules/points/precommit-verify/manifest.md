---
title: Pre-Commit Verification
when: before running precommit-verify, judging its red in a shared checkout, or running the tracked-build check after a commit script
severity: blocking
related: git-safety, commands, completion
---
does-verify-apply ## Does `./le verify current mode full` Apply?
  which-file-types-require-native-verify
  run-verify-when-any-file-in-a-mixed-commit-needs-it
running-the-gate ## Running The Gate
  run-native-verify-and-check-freshness-first
  the-pre-commit-verify-checklist
  run-verify-in-the-foreground-and-wait
  run-the-full-gate-once-when-the-work-is-finished
  run-the-action-that-owns-what-you-changed
  use-the-native-action-or-carry-the-go-build-environment
  which-action-owns-each-surface
  a-wire-behaviour-change-owes-its-functional-suite
  run-the-gates-your-new-files-join
  derive-the-owning-action-when-the-table-has-no-row
  never-commit-with-lint-issues-or-no-test-evidence
  the-spec-completion-and-report-checklist
reading-a-red ## Reading A Red
  read-the-whole-failure-summary-before-re-running
  traps-that-misread-a-verify-log
  give-a-narrow-failure-a-narrow-re-run
  plan-the-first-full-run-to-be-the-last
  do-not-stop-to-ask-which-way
  a-clean-full-verify-is-unreachable-in-a-shared-tree
  run-the-full-gate-before-any-commit-carrying-go
  one-run-covers-every-commit-until-the-next-go-edit
  who-owns-each-red-a-full-run-reports
  a-red-is-dropped-only-when-every-group-names-a-foreign-file
  take-another-sessions-red-as-working-code
  freshness-answers-about-the-paths-you-name
  use-evidence-scoped-to-your-own-files
  the-scoped-evidence-a-shared-checkout-owes
  unverified-is-correct-in-a-shared-checkout
  green-every-structural-gate-even-in-a-shared-tree
  scope-the-gate-to-changes-when-verify-is-known-red
  the-scoped-gates-to-run-instead
  list-only-your-own-files-in-the-commit-script
  attribute-every-red-before-scoping-around-it
  never-let-a-red-persist-across-sessions
what-may-be-overridden ## What May Be Overridden
  never-park-a-deterministic-structural-gate
  structural-red-ok-is-an-owner-only-escape
  a-stale-generated-file-is-structural-not-flaky
  thomas-can-override-the-verify-requirement
  the-two-parts-the-override-must-name
  phrases-that-activate-the-owner-override
  what-the-override-permits-and-forbids
after-the-commit ## After The Commit
  only-one-check-compiles-what-git-holds
  commit-the-producer-with-its-consumer
  clear-a-broken-head-by-committing-the-producer
  the-tracked-build-check-never-reads-test-files
  clear-verification-debt-by-running-the-gate
concurrency ## Concurrency
  run-one-verify-at-a-time-repo-wide
  what-to-do-while-another-verify-holds-the-lock
  a-second-verify-blocks-rather-than-overlaps
  never-edit-the-tree-while-a-verify-runs
