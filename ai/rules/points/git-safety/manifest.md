---
title: Git Safety
when: before any git operation, and when writing or running a commit script
severity: blocking
---
directives ## Directives
  commit-only-through-the-native-command
  push-only-on-the-owners-order
  land-each-chunk-as-it-finishes
  stay-on-your-branch-and-integrate-by-rebase
  never-disable-gpg-signing-or-a-hook
  use-tea-for-pull-requests-and-issues
before-destructive-actions ## Before Destructive Actions
  never-run-a-destructive-git-verb
