---
title: Never Destroy Uncommitted Work
when: before deleting, reverting, or overwriting any file holding uncommitted or user-visible work
severity: blocking
---
directives ## Directives
  never-destroy-uncommitted-work-without-permission
  this-rule-extends-git-safety-to-the-filesystem
forbidden-without-explicit-permission ## Forbidden Without Explicit Permission
  ask-before-deleting-or-overwriting-user-work
what-counts-as-work-the-user-paid-for ## What Counts as "Work the User Paid For"
  treat-these-files-as-user-work
  files-that-do-not-count-as-user-work
bad-reasoning-that-triggers-this-rule ## Bad Reasoning That Triggers This Rule
  why-the-excuses-are-written-down
  excuses-for-destroying-work-and-why-each-fails
when-deletion-is-the-correct-fix ## When Deletion Is The Correct Fix
  ask-for-permission-rather-than-leaving-the-file
  state-the-situation-and-ask-when-unsure
