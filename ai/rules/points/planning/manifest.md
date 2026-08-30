---
title: Specs and Phases
when: before implementing any non-trivial feature, and whenever a spec phase starts, resumes, or closes
severity: blocking
related: completion, quality
---
directives ## Directives
  supervise-delegate-and-review-independently
spec-work-runs-in-subagents-the-main-thread-supervises ## Spec Work Runs in Subagents; the Main Thread Supervises
  the-main-thread-launches-verifies-and-gates-each-phase
  announce-the-fan-out-before-you-spawn-it
  where-each-spec-phase-runs
  launch-independent-phases-in-one-message
  brief-every-subagent-with-its-spec-phase-and-rules
  verify-a-subagent-s-report-against-source
  keep-anything-the-user-must-answer-in-the-main-thread
  delegation-never-dilutes-the-independence-of-review
  delegation-does-not-override-phase-to-model-boundaries
  supervise-thinly-and-keep-exploration-out-of-main
  hand-off-when-the-main-thread-context-grows-too-large
  spawn-one-agent-per-implementation-phase
  report-an-oversized-package-never-trim-its-scope
  banned-delegation-reasoning-and-the-reality
  spawning-an-agent-here-needs-no-permission
^work-phases ## Work Phases
  work-is-classified-by-phase-not-by-convenience
  what-each-work-phase-covers
  implementation-carries-no-model-requirement
  run-every-review-on-opus-5
  review-is-independent-of-the-author
  what-to-do-at-each-phase-boundary
  split-implementation-and-closure-over-two-sessions
  which-session-does-what-in-a-two-session-handoff
  never-downgrade-a-subagent-s-model
  banned-model-phase-reasoning-and-the-reality
  how-the-model-phase-gates-work-and-where-they-stop
spec-selection ## Spec Selection
  one-spec-at-a-time-per-session
plan-file-location ## Plan File Location
  prefer-a-spec-over-a-plan-file
creating-a-spec-blocking ## Creating a Spec (BLOCKING)
  always-start-a-spec-from-plan-template-md
  keep-the-design-and-closure-templates-apart
  leave-placeholders-only-while-the-spec-is-a-skeleton
  name-the-native-verify-action-as-the-spec-s-only-gate
pre-implementation ## Pre-Implementation
  read-the-source-before-the-spec-and-the-spec-before-the-code
implementation-plan-format ## Implementation Plan Format
  present-the-implementation-plan-before-writing-code
  wait-for-user-approval-before-editing-any-file
spec-rules ## Spec Rules
  how-to-write-and-edit-a-spec
risks-assumptions-blocking-for-new-specs ## Risks & Assumptions (BLOCKING for new specs)
  write-gate-concerns-into-the-risks-and-assumptions-tables
  what-the-assumptions-and-risks-tables-capture
  give-every-assumption-a-validation-method
spec-sets ## Spec Sets
  use-a-shared-prefix-for-a-related-set-of-specs
  how-a-spec-set-is-numbered-and-cross-referenced
spec-metadata-blocking ## Spec Metadata (BLOCKING)
  give-every-spec-a-metadata-table-under-its-title
  transition-status-at-the-start-of-a-phase
  update-spec-status-at-each-transition
pre-spec-verification ## Pre-Spec Verification
  complete-the-pre-spec-checklist-before-presenting-a-design
retroactive-specs ## Retroactive Specs
  close-a-retroactive-spec-with-its-own-code
completion-checklist ## Completion Checklist
  finish-these-before-presenting-the-work
review-gate-blocking ## Review Gate (BLOCKING)
  run-the-review-and-loop-until-only-notes-remain
  every-review-round-is-smaller-and-the-loop-must-end
critical-review-is-the-central-deliverable ## Critical Review Is the Central Deliverable
  run-the-review-in-a-different-context-than-the-author
  your-own-reasoning-about-your-code-is-not-a-review
  what-makes-a-review-pass-real
  how-each-review-round-is-scoped-and-when-it-ends
  a-finding-in-the-record-is-not-a-finding-in-the-product
  a-defect-in-test-only-code-is-not-a-finding-in-the-product
  announce-the-pass-count-and-lenses-before-spawning
  the-commit-gate-that-needs-a-clean-review-artifact
  what-the-review-hook-can-and-cannot-prove
  banned-review-rationalizations-and-the-reality
  review-every-closure-and-every-substantive-change
spec-closure-blocking ## Spec Closure (BLOCKING)
  a-spec-is-not-done-until-it-is-deleted-from-plan
  the-spec-closure-lifecycle
  why-closure-needs-two-commits
  produce-both-closure-commits-from-one-script
  repoint-every-reference-to-the-spec-not-only-design
  re-read-what-a-bulk-repoint-produced
  baseline-a-citation-only-when-it-is-a-historical-record
  resolve-rows-pointing-at-the-spec-during-closure
  why-closure-must-resolve-rows-that-name-the-spec
  never-resolve-a-row-to-a-record-that-omits-the-item
  never-git-rm-f-an-uncommitted-spec
  banned-closure-shortcuts-and-why
spec-preservation ## Spec Preservation
  what-to-discard-when-writing-the-row
  delete-the-spec-only-after-the-journal-row-is-written
  commit-the-spec-before-deletion
verify-specs-against-code-blocking ## Verify Specs Against Code (BLOCKING)
  grep-the-code-before-reporting-spec-progress
deferred-work-blocking ## Deferred Work (BLOCKING)
  verify-every-deferral-before-marking-a-spec-done
spec-triage ## Spec Triage
  rule
  judge-the-tree-not-the-spec
  an-improvement-leaves-the-release-backlog
deferral-tracking ## Deferral Tracking
  record-and-home-every-scope-reduction
  a-dedicated-spec-replaces-the-row
  which-shard-holds-a-row
  delete-a-shard-at-closure-only-when-every-row-is-done
  read-the-closing-spec-s-own-shard-as-a-source
  keep-a-shard-that-still-holds-a-live-row
  a-live-row-beats-deletion-at-closure
  never-bulk-delete-orphaned-shards
  the-gate-that-blocks-removing-a-live-bearing-shard
  the-last-closer-deletes-an-all-terminal-orphan-shard
  give-an-orphaned-live-row-a-real-destination
  what-triggers-a-deferral-record
  the-deferral-recording-rules
  keep-a-homed-deferral-row-live-until-the-work-lands
  never-close-a-row-early-to-quiet-the-warning
  the-blind-spot-a-terminal-status-creates
  never-home-a-deferral-at-plan-known-failures
  give-every-deferral-a-destination-spec-immediately
  how-to-choose-a-destination-spec-in-order
  prefer-an-existing-spec-over-a-new-deferral-file
  record-a-live-row-whichever-route-homes-the-work
  rules-for-naming-a-deferral-holder-spec
  grep-before-claiming-infrastructure-is-missing
  what-does-not-count-as-a-deferral
  close-a-row-when-the-work-lands-not-when-it-is-filed
  filing-work-in-a-spec-does-not-close-the-row
executive-summary-report ## Executive Summary Report
  present-the-executive-summary-when-work-is-complete
  treat-the-sections-as-a-checklist-not-a-quota
documentation-update-checklist-blocking ## Documentation Update Checklist (BLOCKING)
  where-the-canonical-documentation-checklist-lives
  a-new-page-or-a-changed-claim-owes-an-independent-reader
writing-journal-rows ## Writing Journal Rows
  write-a-row-only-when-the-work-taught-something
  the-quality-test-every-journal-row-must-pass
session-handoff ## Session Handoff
  start-a-handoff-with-its-rationale-then-the-edits
  the-rationale-is-a-verification-checkpoint-not-an-essay
  write-a-handoff-to-plan-handover-when-it-must-persist
  rules-for-naming-and-placing-a-handover-file
  rules-for-the-edits-in-a-handoff
